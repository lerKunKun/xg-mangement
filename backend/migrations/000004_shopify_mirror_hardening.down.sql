ALTER TABLE shopify_themes DROP CONSTRAINT IF EXISTS shopify_themes_role_check;
UPDATE shopify_themes SET role = 'UNPUBLISHED' WHERE role IN ('ARCHIVED', 'LOCKED');
ALTER TABLE shopify_themes ADD CONSTRAINT shopify_themes_role_check
    CHECK (role IN ('MAIN', 'UNPUBLISHED', 'DEVELOPMENT', 'DEMO', 'MOBILE'));

ALTER TABLE webhook_events DROP CONSTRAINT IF EXISTS webhook_events_store_tenant_fk;
ALTER TABLE shopify_resource_snapshots DROP CONSTRAINT IF EXISTS shopify_snapshots_store_tenant_fk;
ALTER TABLE shopify_themes DROP CONSTRAINT IF EXISTS shopify_themes_store_tenant_fk;
ALTER TABLE shopify_collections DROP CONSTRAINT IF EXISTS shopify_collections_store_tenant_fk;
ALTER TABLE shopify_variants
    DROP CONSTRAINT IF EXISTS shopify_variants_product_tenant_fk,
    DROP CONSTRAINT IF EXISTS shopify_variants_store_tenant_fk;
ALTER TABLE shopify_products
    DROP CONSTRAINT IF EXISTS shopify_products_store_tenant_fk,
    DROP CONSTRAINT IF EXISTS shopify_products_org_store_id_unique;
DROP INDEX IF EXISTS shopify_sync_runs_expired_lease_idx;
ALTER TABLE shopify_sync_runs
    DROP CONSTRAINT IF EXISTS shopify_sync_runs_store_tenant_fk,
    DROP COLUMN IF EXISTS heartbeat_at,
    DROP COLUMN IF EXISTS lease_expires_at,
    DROP COLUMN IF EXISTS lease_owner;
ALTER TABLE shopify_stores DROP CONSTRAINT IF EXISTS shopify_stores_account_tenant_fk;
ALTER TABLE integration_accounts DROP CONSTRAINT IF EXISTS integration_accounts_org_id_unique;
ALTER TABLE shopify_stores
    DROP CONSTRAINT IF EXISTS shopify_stores_org_id_unique,
    DROP COLUMN IF EXISTS resync_requested;
DROP INDEX IF EXISTS shopify_stores_domain_global_unique;
