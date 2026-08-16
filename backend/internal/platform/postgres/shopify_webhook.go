package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/xg-management/platform/backend/internal/httpapi"
	"github.com/xg-management/platform/backend/internal/shopifysync"
)

func (c *Client) ResolveShopifyWebhookTarget(ctx context.Context, domain string) (httpapi.ShopifyWebhookTarget, error) {
	var target httpapi.ShopifyWebhookTarget
	err := c.pool.QueryRow(ctx, `
		SELECT s.organization_id::text,s.id::text,cfg.encrypted_secrets
		FROM shopify_stores s
		JOIN integration_configs cfg ON cfg.organization_id=s.organization_id AND cfg.provider='shopify' AND cfg.enabled=true
		WHERE lower(s.shop_domain)=lower($1) AND s.status='connected'`, domain).
		Scan(&target.OrganizationID, &target.StoreID, &target.EncryptedSecrets)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.ShopifyWebhookTarget{}, httpapi.ErrShopifyWebhookStoreNotFound
	}
	if err != nil {
		return httpapi.ShopifyWebhookTarget{}, fmt.Errorf("resolve Shopify webhook target: %w", err)
	}
	return target, nil
}

func (c *Client) RecordShopifyEventAndOutbox(ctx context.Context, event httpapi.ShopifyWebhookEvent) (bool, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin Shopify webhook transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `
		INSERT INTO webhook_events(provider,event_id,organization_id,store_id,topic,processing_status)
		VALUES('shopify',$1,$2,$3,$4,'recorded') ON CONFLICT(provider,event_id) DO NOTHING`,
		event.WebhookID, event.OrganizationID, event.StoreID, event.Topic)
	if err != nil {
		return false, fmt.Errorf("record Shopify webhook: %w", err)
	}
	if command.RowsAffected() == 0 {
		return true, nil
	}
	if event.Topic == "app/uninstalled" {
		if err := disconnectWebhookStore(ctx, tx, event); err != nil {
			return false, err
		}
	} else if topicTriggersMirror(event.Topic) {
		if err := enqueueWebhookSync(ctx, tx, event); err != nil {
			return false, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE webhook_events SET processed_at=now(),processing_status='processed' WHERE provider='shopify' AND event_id=$1`, event.WebhookID); err != nil {
		return false, fmt.Errorf("complete Shopify webhook record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit Shopify webhook transaction: %w", err)
	}
	return false, nil
}

func disconnectWebhookStore(ctx context.Context, tx pgx.Tx, event httpapi.ShopifyWebhookEvent) error {
	var accountID *string
	if err := tx.QueryRow(ctx, `
		UPDATE shopify_stores SET status='disconnected',updated_at=now()
		WHERE organization_id=$1 AND id=$2 RETURNING integration_account_id::text`, event.OrganizationID, event.StoreID).Scan(&accountID); err != nil {
		return fmt.Errorf("disconnect uninstalled Shopify store: %w", err)
	}
	if accountID != nil {
		if _, err := tx.Exec(ctx, `UPDATE integration_accounts SET status='disconnected',encrypted_credentials=NULL,updated_at=now() WHERE organization_id=$1 AND id=$2`, event.OrganizationID, *accountID); err != nil {
			return fmt.Errorf("disconnect uninstalled Shopify account: %w", err)
		}
	}
	return nil
}

func enqueueWebhookSync(ctx context.Context, tx pgx.Tx, event httpapi.ShopifyWebhookEvent) error {
	var storeStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM shopify_stores WHERE organization_id=$1 AND id=$2 FOR UPDATE`, event.OrganizationID, event.StoreID).Scan(&storeStatus); err != nil {
		return fmt.Errorf("lock webhook Shopify store: %w", err)
	}
	if storeStatus != "connected" {
		return nil
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shopify_sync_runs WHERE organization_id=$1 AND store_id=$2 AND status IN ('queued','running'))`, event.OrganizationID, event.StoreID).Scan(&active); err != nil {
		return fmt.Errorf("check active webhook Shopify sync: %w", err)
	}
	if active {
		if _, err := tx.Exec(ctx, `UPDATE shopify_stores SET resync_requested=true,updated_at=now() WHERE organization_id=$1 AND id=$2`, event.OrganizationID, event.StoreID); err != nil {
			return fmt.Errorf("request Shopify follow-up sync: %w", err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE shopify_stores SET resync_requested=false WHERE organization_id=$1 AND id=$2`, event.OrganizationID, event.StoreID); err != nil {
		return fmt.Errorf("clear stale Shopify follow-up sync request: %w", err)
	}
	return enqueueSyncRunTx(ctx, tx, event.OrganizationID, event.StoreID, "shopify-webhook:"+event.WebhookID, shopifysync.SyncMode(webhookSyncMode(event.Topic)), "shopify-webhook:")
}

// Webhooks currently schedule a full reconciliation because the Bulk queries
// return complete collections. This keeps delete events correct until a truly
// targeted incremental query and explicit deletion set are implemented.
func webhookSyncMode(string) string { return "full" }

func topicTriggersMirror(topic string) bool {
	switch topic {
	case "products/create", "products/update", "products/delete",
		"collections/create", "collections/update", "collections/delete", "themes/publish":
		return true
	default:
		return false
	}
}
