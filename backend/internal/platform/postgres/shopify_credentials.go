package postgres

import (
	"context"
	"fmt"

	"github.com/xg-management/platform/backend/internal/shopifysync"
)

// WithLockedConnection serializes token rotation per store. A state update is
// committed even when the callback returns an error so permanent provider
// failures such as reauthorization_required remain visible to operators.
func (c *Client) WithLockedConnection(ctx context.Context, organizationID, storeID string, fn func(shopifysync.StoreConnection) (shopifysync.CredentialUpdate, error)) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Shopify credential transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var connection shopifysync.StoreConnection
	err = tx.QueryRow(ctx, `
		SELECT s.organization_id::text, s.id::text, a.id::text, s.shop_domain, s.status,
		       a.encrypted_credentials, a.expires_at, a.refresh_expires_at,
		       cfg.public_config, cfg.encrypted_secrets, COALESCE(a.last_error, ''), COALESCE(a.scopes, '{}')
		FROM shopify_stores s
		JOIN integration_accounts a ON a.id = s.integration_account_id
		JOIN integration_configs cfg ON cfg.organization_id = s.organization_id
		    AND cfg.provider = 'shopify' AND cfg.enabled = true
		WHERE s.organization_id = $1 AND s.id = $2 AND a.provider = 'shopify'
		FOR UPDATE OF a, s`, organizationID, storeID).Scan(
		&connection.OrganizationID, &connection.StoreID, &connection.AccountID,
		&connection.Domain, &connection.Status, &connection.EncryptedCredentials,
		&connection.ExpiresAt, &connection.RefreshExpiresAt, &connection.PublicConfig,
		&connection.EncryptedSecrets, &connection.LastErrorCode, &connection.GrantedScopes,
	)
	if err != nil {
		return fmt.Errorf("lock Shopify connection: %w", err)
	}

	update, callbackErr := fn(connection)
	if update.Changed {
		if update.Status == "" {
			update.Status = connection.Status
		}
		_, err = tx.Exec(ctx, `
			UPDATE integration_accounts
			SET encrypted_credentials = CASE WHEN $3::bytea IS NULL THEN encrypted_credentials ELSE $3 END,
			    expires_at = CASE WHEN $3::bytea IS NULL THEN expires_at ELSE $4 END,
			    refresh_expires_at = CASE WHEN $3::bytea IS NULL THEN refresh_expires_at ELSE $5 END,
			    status = $6, last_error = NULLIF($7, ''), updated_at = now()
			WHERE organization_id = $1 AND id = $2`, organizationID, connection.AccountID,
			nullBytes(update.EncryptedCredentials), update.ExpiresAt, update.RefreshExpiresAt,
			update.Status, update.LastErrorCode)
		if err != nil {
			return fmt.Errorf("update Shopify credentials: %w", err)
		}
		if _, err = tx.Exec(ctx, `UPDATE shopify_stores SET status=$3, updated_at=now() WHERE organization_id=$1 AND id=$2`, organizationID, storeID, update.Status); err != nil {
			return fmt.Errorf("update Shopify store credential status: %w", err)
		}
	}
	if callbackErr != nil && !update.Changed {
		return callbackErr
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Shopify credential transaction: %w", err)
	}
	return callbackErr
}

func nullBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func (c *Client) ResolveSyncTarget(ctx context.Context, organizationID, storeID string) (shopifysync.SyncTarget, error) {
	var target shopifysync.SyncTarget
	err := c.pool.QueryRow(ctx, `
		SELECT s.shop_domain
		FROM shopify_stores s
		JOIN integration_configs cfg ON cfg.organization_id=s.organization_id AND cfg.provider='shopify' AND cfg.enabled=true
		WHERE s.organization_id=$1 AND s.id=$2 AND s.status='connected'`, organizationID, storeID).
		Scan(&target.Domain)
	if err != nil {
		return shopifysync.SyncTarget{}, fmt.Errorf("resolve Shopify sync target: %w", err)
	}
	target.APIVersion = "2026-07"
	return target, nil
}
