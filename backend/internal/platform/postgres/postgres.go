package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xg-management/platform/backend/internal/httpapi"
)

type Client struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, databaseURL string) (*Client, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return &Client{pool: pool}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

func (c *Client) Close() {
	c.pool.Close()
}

func (c *Client) List(ctx context.Context, organizationID string) ([]httpapi.Store, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id::text, name, shop_domain, status, COALESCE(last_synced_at::text, '')
		FROM shopify_stores
		WHERE organization_id = $1
		ORDER BY created_at DESC`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("query Shopify stores: %w", err)
	}
	defer rows.Close()

	stores := make([]httpapi.Store, 0)
	for rows.Next() {
		var store httpapi.Store
		if err := rows.Scan(&store.ID, &store.Name, &store.Domain, &store.Status, &store.LastSync); err != nil {
			return nil, fmt.Errorf("scan Shopify store: %w", err)
		}
		stores = append(stores, store)
	}
	return stores, rows.Err()
}

func (c *Client) ListAssets(ctx context.Context, organizationID string) ([]httpapi.Asset, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id::text, name, object_key, content_type, status
		FROM assets
		WHERE organization_id = $1
		ORDER BY created_at DESC`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("query assets: %w", err)
	}
	defer rows.Close()

	assets := make([]httpapi.Asset, 0)
	for rows.Next() {
		var asset httpapi.Asset
		if err := rows.Scan(&asset.ID, &asset.Name, &asset.ObjectKey, &asset.ContentType, &asset.Status); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (c *Client) ListApprovals(ctx context.Context, organizationID string) ([]httpapi.Approval, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT id::text, request_type, status, COALESCE(dingtalk_instance_id, '')
		FROM approval_requests
		WHERE organization_id = $1
		ORDER BY created_at DESC`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("query approvals: %w", err)
	}
	defer rows.Close()

	approvals := make([]httpapi.Approval, 0)
	for rows.Next() {
		var approval httpapi.Approval
		if err := rows.Scan(&approval.ID, &approval.Type, &approval.Status, &approval.DingTalkInstanceID); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		approvals = append(approvals, approval)
	}
	return approvals, rows.Err()
}

type AssetRepository struct{ client *Client }

func NewAssetRepository(client *Client) AssetRepository {
	return AssetRepository{client: client}
}

func (r AssetRepository) List(ctx context.Context, organizationID string) ([]httpapi.Asset, error) {
	return r.client.ListAssets(ctx, organizationID)
}

type ApprovalRepository struct{ client *Client }

func NewApprovalRepository(client *Client) ApprovalRepository {
	return ApprovalRepository{client: client}
}

func (r ApprovalRepository) List(ctx context.Context, organizationID string) ([]httpapi.Approval, error) {
	return r.client.ListApprovals(ctx, organizationID)
}
