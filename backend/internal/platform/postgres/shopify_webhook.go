package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xg-management/platform/backend/internal/httpapi"
	"github.com/xg-management/platform/backend/internal/jobs"
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
	jobID := "shopify-webhook:" + event.WebhookID
	var runID string
	err := tx.QueryRow(ctx, `
		INSERT INTO shopify_sync_runs(organization_id,store_id,mode,status,job_id)
		SELECT $1,$2,'incremental','queued',$3
		WHERE NOT EXISTS(SELECT 1 FROM shopify_sync_runs WHERE organization_id=$1 AND store_id=$2 AND status IN ('queued','running'))
		RETURNING id::text`, event.OrganizationID, event.StoreID, jobID).Scan(&runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create webhook Shopify sync run: %w", err)
	}
	payload, err := json.Marshal(map[string]string{"store_id": event.StoreID, "run_id": runID, "mode": "incremental"})
	if err != nil {
		return fmt.Errorf("encode webhook sync payload: %w", err)
	}
	envelope := jobs.Envelope{Version: 1, ID: jobID, Type: jobs.TypeShopifyStoreSyncRequested, OrganizationID: event.OrganizationID, OccurredAt: time.Now().UTC(), Payload: payload}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode webhook sync envelope: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_messages(organization_id,event_key,envelope) VALUES($1,$2,$3) ON CONFLICT(event_key) DO NOTHING`, event.OrganizationID, jobID, envelopeJSON); err != nil {
		return fmt.Errorf("enqueue webhook sync outbox: %w", err)
	}
	return nil
}

func topicTriggersMirror(topic string) bool {
	switch topic {
	case "products/create", "products/update", "products/delete",
		"collections/create", "collections/update", "collections/delete", "themes/publish":
		return true
	default:
		return false
	}
}
