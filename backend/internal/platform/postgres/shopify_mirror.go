package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xg-management/platform/backend/internal/jobs"
	"github.com/xg-management/platform/backend/internal/shopifysync"
)

func normalizeMirrorBatch(batch shopifysync.MirrorBatch) (shopifysync.MirrorBatch, error) {
	if batch.SyncedAt.IsZero() {
		batch.SyncedAt = time.Now().UTC()
	}
	products := make(map[string]shopifysync.Product, len(batch.Products))
	for _, item := range batch.Products {
		item.ShopifyGID = strings.TrimSpace(item.ShopifyGID)
		if item.ShopifyGID == "" {
			return shopifysync.MirrorBatch{}, fmt.Errorf("Shopify product GID is required")
		}
		item.Payload = validPayload(item.Payload)
		if previous, ok := products[item.ShopifyGID]; !ok || resourceIsNewer(previous.SourceUpdatedAt, item.SourceUpdatedAt) {
			products[item.ShopifyGID] = item
		}
	}
	variants := make(map[string]shopifysync.Variant, len(batch.Variants))
	for _, item := range batch.Variants {
		item.ShopifyGID = strings.TrimSpace(item.ShopifyGID)
		item.ProductShopifyGID = strings.TrimSpace(item.ProductShopifyGID)
		if item.ShopifyGID == "" || item.ProductShopifyGID == "" {
			return shopifysync.MirrorBatch{}, fmt.Errorf("Shopify variant and parent product GIDs are required")
		}
		if _, ok := products[item.ProductShopifyGID]; !ok {
			return shopifysync.MirrorBatch{}, fmt.Errorf("Shopify variant %s references missing product %s", item.ShopifyGID, item.ProductShopifyGID)
		}
		item.Payload = validPayload(item.Payload)
		if previous, ok := variants[item.ShopifyGID]; !ok || resourceIsNewer(previous.SourceUpdatedAt, item.SourceUpdatedAt) {
			variants[item.ShopifyGID] = item
		}
	}
	collections := make(map[string]shopifysync.Collection, len(batch.Collections))
	for _, item := range batch.Collections {
		item.ShopifyGID = strings.TrimSpace(item.ShopifyGID)
		if item.ShopifyGID == "" {
			return shopifysync.MirrorBatch{}, fmt.Errorf("Shopify collection GID is required")
		}
		item.Payload = validPayload(item.Payload)
		if previous, ok := collections[item.ShopifyGID]; !ok || resourceIsNewer(previous.SourceUpdatedAt, item.SourceUpdatedAt) {
			collections[item.ShopifyGID] = item
		}
	}
	themes := make(map[string]shopifysync.Theme, len(batch.Themes))
	for _, item := range batch.Themes {
		item.ShopifyGID = strings.TrimSpace(item.ShopifyGID)
		if item.ShopifyGID == "" {
			return shopifysync.MirrorBatch{}, fmt.Errorf("Shopify theme GID is required")
		}
		if previous, ok := themes[item.ShopifyGID]; !ok || resourceIsNewer(previous.UpdatedAt, item.UpdatedAt) {
			themes[item.ShopifyGID] = item
		}
	}

	result := shopifysync.MirrorBatch{SyncedAt: batch.SyncedAt}
	for _, item := range products {
		result.Products = append(result.Products, item)
	}
	for _, item := range variants {
		result.Variants = append(result.Variants, item)
	}
	for _, item := range collections {
		result.Collections = append(result.Collections, item)
	}
	for _, item := range themes {
		result.Themes = append(result.Themes, item)
	}
	sort.Slice(result.Products, func(i, j int) bool { return result.Products[i].ShopifyGID < result.Products[j].ShopifyGID })
	sort.Slice(result.Variants, func(i, j int) bool { return result.Variants[i].ShopifyGID < result.Variants[j].ShopifyGID })
	sort.Slice(result.Collections, func(i, j int) bool { return result.Collections[i].ShopifyGID < result.Collections[j].ShopifyGID })
	sort.Slice(result.Themes, func(i, j int) bool { return result.Themes[i].ShopifyGID < result.Themes[j].ShopifyGID })
	return result, nil
}

func resourceIsNewer(previous, candidate *time.Time) bool {
	if previous == nil {
		return true
	}
	return candidate != nil && candidate.After(*previous)
}

func validPayload(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 || !json.Valid(payload) {
		return json.RawMessage(`{}`)
	}
	return payload
}

func (c *Client) StartSyncRun(ctx context.Context, request shopifysync.SyncRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	command, err := c.pool.Exec(ctx, `
		UPDATE shopify_sync_runs SET status='running', started_at=COALESCE(started_at, now()),
		    completed_at=NULL, error_code=NULL, error_message=NULL
		WHERE organization_id=$1 AND store_id=$2 AND id=$3 AND status IN ('queued','failed')`,
		request.OrganizationID, request.StoreID, request.RunID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return shopifysync.ErrSyncAlreadyRunning
		}
		return fmt.Errorf("start Shopify sync run: %w", err)
	}
	if command.RowsAffected() == 0 {
		var status shopifysync.SyncStatus
		err := c.pool.QueryRow(ctx, `SELECT status FROM shopify_sync_runs WHERE organization_id=$1 AND store_id=$2 AND id=$3`, request.OrganizationID, request.StoreID, request.RunID).Scan(&status)
		if err != nil {
			return fmt.Errorf("find Shopify sync run state: %w", err)
		}
		switch status {
		case shopifysync.SyncStatusCompleted:
			return shopifysync.ErrSyncAlreadyCompleted
		case shopifysync.SyncStatusRunning:
			return shopifysync.ErrSyncAlreadyRunning
		default:
			return fmt.Errorf("Shopify sync run cannot start from status %q", status)
		}
	}
	return nil
}

func (c *Client) CreateSyncRun(ctx context.Context, organizationID, storeID, requestedBy string, mode shopifysync.SyncMode) (shopifysync.SyncRun, error) {
	if mode == "" {
		mode = shopifysync.SyncModeFull
	}
	if mode != shopifysync.SyncModeFull && mode != shopifysync.SyncModeIncremental {
		return shopifysync.SyncRun{}, fmt.Errorf("unsupported sync mode %q", mode)
	}
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return shopifysync.SyncRun{}, fmt.Errorf("begin Shopify sync request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var run shopifysync.SyncRun
	err = tx.QueryRow(ctx, `
		INSERT INTO shopify_sync_runs(organization_id,store_id,mode,status,requested_by,job_id)
		SELECT $1,s.id,$3,'queued',$4,gen_random_uuid()::text
		FROM shopify_stores s WHERE s.organization_id=$1 AND s.id=$2 AND s.status='connected'
		RETURNING id::text,store_id::text,mode,status,job_id,created_at`, organizationID, storeID, mode, requestedBy).
		Scan(&run.ID, &run.StoreID, &run.Mode, &run.Status, &run.JobID, &run.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return shopifysync.SyncRun{}, shopifysync.ErrSyncAlreadyRunning
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return shopifysync.SyncRun{}, fmt.Errorf("connected Shopify store was not found")
		}
		return shopifysync.SyncRun{}, fmt.Errorf("create Shopify sync run: %w", err)
	}
	payload, err := json.Marshal(map[string]any{"store_id": run.StoreID, "run_id": run.ID, "mode": run.Mode})
	if err != nil {
		return shopifysync.SyncRun{}, err
	}
	envelope := jobs.Envelope{Version: 1, ID: run.JobID, Type: jobs.TypeShopifyStoreSyncRequested, OrganizationID: organizationID, OccurredAt: run.CreatedAt, Payload: payload}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return shopifysync.SyncRun{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO outbox_messages(organization_id,event_key,envelope) VALUES($1,$2,$3)`, organizationID, "shopify-sync:"+run.ID, envelopeJSON); err != nil {
		return shopifysync.SyncRun{}, fmt.Errorf("enqueue Shopify sync run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return shopifysync.SyncRun{}, fmt.Errorf("commit Shopify sync request: %w", err)
	}
	return run, nil
}

func (c *Client) ReplaceMirror(ctx context.Context, request shopifysync.SyncRequest, input shopifysync.MirrorBatch) error {
	if err := request.Validate(); err != nil {
		return err
	}
	batch, err := normalizeMirrorBatch(input)
	if err != nil {
		return err
	}
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Shopify mirror transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	productIDs := make(map[string]string, len(batch.Products))
	for _, item := range batch.Products {
		var productID string
		err = tx.QueryRow(ctx, `
			INSERT INTO shopify_products(organization_id,store_id,shopify_gid,handle,title,status,vendor,product_type,payload,source_updated_at,synced_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT(organization_id,store_id,shopify_gid) DO UPDATE SET
				handle=EXCLUDED.handle,title=EXCLUDED.title,status=EXCLUDED.status,vendor=EXCLUDED.vendor,
				product_type=EXCLUDED.product_type,payload=EXCLUDED.payload,source_updated_at=EXCLUDED.source_updated_at,synced_at=EXCLUDED.synced_at
			RETURNING id::text`, request.OrganizationID, request.StoreID, item.ShopifyGID, item.Handle,
			item.Title, item.Status, item.Vendor, item.ProductType, item.Payload, item.SourceUpdatedAt, batch.SyncedAt).Scan(&productID)
		if err != nil {
			return fmt.Errorf("upsert Shopify product %s: %w", item.ShopifyGID, err)
		}
		productIDs[item.ShopifyGID] = productID
	}
	for _, item := range batch.Variants {
		_, err = tx.Exec(ctx, `
			INSERT INTO shopify_variants(organization_id,store_id,product_id,shopify_gid,sku,title,price,compare_at_price,inventory_quantity,payload,source_updated_at,synced_at)
			VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::numeric,NULLIF($8,'')::numeric,$9,$10,$11,$12)
			ON CONFLICT(organization_id,store_id,shopify_gid) DO UPDATE SET
				product_id=EXCLUDED.product_id,sku=EXCLUDED.sku,title=EXCLUDED.title,price=EXCLUDED.price,
				compare_at_price=EXCLUDED.compare_at_price,inventory_quantity=EXCLUDED.inventory_quantity,
				payload=EXCLUDED.payload,source_updated_at=EXCLUDED.source_updated_at,synced_at=EXCLUDED.synced_at`,
			request.OrganizationID, request.StoreID, productIDs[item.ProductShopifyGID], item.ShopifyGID,
			item.SKU, item.Title, item.Price, item.CompareAtPrice, item.InventoryQuantity, item.Payload, item.SourceUpdatedAt, batch.SyncedAt)
		if err != nil {
			return fmt.Errorf("upsert Shopify variant %s: %w", item.ShopifyGID, err)
		}
	}
	for _, item := range batch.Collections {
		_, err = tx.Exec(ctx, `
			INSERT INTO shopify_collections(organization_id,store_id,shopify_gid,handle,title,collection_type,products_count,payload,source_updated_at,synced_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT(organization_id,store_id,shopify_gid) DO UPDATE SET
				handle=EXCLUDED.handle,title=EXCLUDED.title,collection_type=EXCLUDED.collection_type,
				products_count=EXCLUDED.products_count,payload=EXCLUDED.payload,source_updated_at=EXCLUDED.source_updated_at,synced_at=EXCLUDED.synced_at`,
			request.OrganizationID, request.StoreID, item.ShopifyGID, item.Handle, item.Title,
			item.CollectionType, item.ProductsCount, item.Payload, item.SourceUpdatedAt, batch.SyncedAt)
		if err != nil {
			return fmt.Errorf("upsert Shopify collection %s: %w", item.ShopifyGID, err)
		}
	}
	for _, item := range batch.Themes {
		_, err = tx.Exec(ctx, `
			INSERT INTO shopify_themes(organization_id,store_id,shopify_gid,name,role,processing,processing_failed,theme_store_id,source_release_id,source_updated_at,synced_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT(organization_id,store_id,shopify_gid) DO UPDATE SET
				name=EXCLUDED.name,role=EXCLUDED.role,processing=EXCLUDED.processing,
				processing_failed=EXCLUDED.processing_failed,theme_store_id=EXCLUDED.theme_store_id,
				source_release_id=EXCLUDED.source_release_id,source_updated_at=EXCLUDED.source_updated_at,synced_at=EXCLUDED.synced_at`,
			request.OrganizationID, request.StoreID, item.ShopifyGID, item.Name, item.Role, item.Processing,
			item.ProcessingFailed, item.ThemeStoreID, item.SourceReleaseID, item.UpdatedAt, batch.SyncedAt)
		if err != nil {
			return fmt.Errorf("upsert Shopify theme %s: %w", item.ShopifyGID, err)
		}
	}
	if request.Mode != shopifysync.SyncModeIncremental {
		if err := removeMissingMirrorRows(ctx, tx, request, batch); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE shopify_stores SET last_synced_at=$3, updated_at=now() WHERE organization_id=$1 AND id=$2`, request.OrganizationID, request.StoreID, batch.SyncedAt); err != nil {
		return fmt.Errorf("mark Shopify store synchronized: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit Shopify mirror transaction: %w", err)
	}
	return nil
}

func removeMissingMirrorRows(ctx context.Context, tx pgx.Tx, request shopifysync.SyncRequest, batch shopifysync.MirrorBatch) error {
	productGIDs := productGIDs(batch.Products)
	variantGIDs := variantGIDs(batch.Variants)
	collectionGIDs := collectionGIDs(batch.Collections)
	themeGIDs := themeGIDs(batch.Themes)
	queries := []struct {
		name  string
		query string
		ids   []string
	}{
		{"variants", `DELETE FROM shopify_variants WHERE organization_id=$1 AND store_id=$2 AND NOT (shopify_gid = ANY($3::text[]))`, variantGIDs},
		{"products", `DELETE FROM shopify_products WHERE organization_id=$1 AND store_id=$2 AND NOT (shopify_gid = ANY($3::text[]))`, productGIDs},
		{"collections", `DELETE FROM shopify_collections WHERE organization_id=$1 AND store_id=$2 AND NOT (shopify_gid = ANY($3::text[]))`, collectionGIDs},
		{"themes", `DELETE FROM shopify_themes WHERE organization_id=$1 AND store_id=$2 AND NOT (shopify_gid = ANY($3::text[]))`, themeGIDs},
	}
	for _, item := range queries {
		if _, err := tx.Exec(ctx, item.query, request.OrganizationID, request.StoreID, item.ids); err != nil {
			return fmt.Errorf("remove missing Shopify %s: %w", item.name, err)
		}
	}
	return nil
}

func (c *Client) CompleteSyncRun(ctx context.Context, request shopifysync.SyncRequest, counts shopifysync.ResourceCounts) error {
	command, err := c.pool.Exec(ctx, `
		UPDATE shopify_sync_runs SET status='completed',products_count=$4,variants_count=$5,
		collections_count=$6,themes_count=$7,error_code=NULL,error_message=NULL,completed_at=now()
		WHERE organization_id=$1 AND store_id=$2 AND id=$3 AND status='running'`, request.OrganizationID,
		request.StoreID, request.RunID, counts.Products, counts.Variants, counts.Collections, counts.Themes)
	if err != nil {
		return fmt.Errorf("complete Shopify sync run: %w", err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("Shopify sync run is not running")
	}
	return nil
}

func (c *Client) FailSyncRun(ctx context.Context, request shopifysync.SyncRequest, code, message string) error {
	_, err := c.pool.Exec(ctx, `
		UPDATE shopify_sync_runs SET status='failed',error_code=$4,error_message=$5,completed_at=now()
		WHERE organization_id=$1 AND store_id=$2 AND id=$3 AND status IN ('queued','running')`,
		request.OrganizationID, request.StoreID, request.RunID, code, message)
	if err != nil {
		return fmt.Errorf("fail Shopify sync run: %w", err)
	}
	return nil
}

func (c *Client) ListSyncRuns(ctx context.Context, organizationID, storeID string, limit int) ([]shopifysync.SyncRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := c.pool.Query(ctx, `
		SELECT id::text,organization_id::text,store_id::text,mode,status,job_id,
		products_count,variants_count,collections_count,themes_count,
		COALESCE(error_code,''),COALESCE(error_message,''),started_at,completed_at,created_at
		FROM shopify_sync_runs WHERE organization_id=$1 AND store_id=$2 ORDER BY created_at DESC LIMIT $3`,
		organizationID, storeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list Shopify sync runs: %w", err)
	}
	defer rows.Close()
	result := make([]shopifysync.SyncRun, 0)
	for rows.Next() {
		var item shopifysync.SyncRun
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.StoreID, &item.Mode, &item.Status, &item.JobID,
			&item.Counts.Products, &item.Counts.Variants, &item.Counts.Collections, &item.Counts.Themes,
			&item.ErrorCode, &item.ErrorMessage, &item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan Shopify sync run: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (c *Client) ListThemes(ctx context.Context, organizationID, storeID string) ([]shopifysync.Theme, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT shopify_gid,name,role,processing,processing_failed,theme_store_id,source_release_id::text,source_updated_at,synced_at
		FROM shopify_themes WHERE organization_id=$1 AND store_id=$2 ORDER BY (role='MAIN') DESC,name`, organizationID, storeID)
	if err != nil {
		return nil, fmt.Errorf("list Shopify themes: %w", err)
	}
	defer rows.Close()
	result := make([]shopifysync.Theme, 0)
	for rows.Next() {
		var item shopifysync.Theme
		if err := rows.Scan(&item.ShopifyGID, &item.Name, &item.Role, &item.Processing, &item.ProcessingFailed,
			&item.ThemeStoreID, &item.SourceReleaseID, &item.UpdatedAt, &item.SyncedAt); err != nil {
			return nil, fmt.Errorf("scan Shopify theme: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func productGIDs(items []shopifysync.Product) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ShopifyGID)
	}
	return result
}
func variantGIDs(items []shopifysync.Variant) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ShopifyGID)
	}
	return result
}
func collectionGIDs(items []shopifysync.Collection) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ShopifyGID)
	}
	return result
}
func themeGIDs(items []shopifysync.Theme) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ShopifyGID)
	}
	return result
}
