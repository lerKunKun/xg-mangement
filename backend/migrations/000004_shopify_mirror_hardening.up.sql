-- A Shopify shop domain is a global identity. Webhooks do not carry our
-- organization ID, so the domain must resolve to exactly one tenant.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM shopify_stores
        GROUP BY lower(shop_domain) HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate Shopify shop domains detected; disconnect duplicate tenant records before applying 000004';
    END IF;
END $$;

CREATE UNIQUE INDEX shopify_stores_domain_global_unique
    ON shopify_stores (lower(shop_domain));

ALTER TABLE shopify_stores
    ADD COLUMN resync_requested BOOLEAN NOT NULL DEFAULT false,
    ADD CONSTRAINT shopify_stores_org_id_unique UNIQUE (organization_id, id);

ALTER TABLE integration_accounts
    ADD CONSTRAINT integration_accounts_org_id_unique UNIQUE (organization_id, id);

ALTER TABLE shopify_stores
    ADD CONSTRAINT shopify_stores_account_tenant_fk
    FOREIGN KEY (organization_id, integration_account_id)
    REFERENCES integration_accounts(organization_id, id);

ALTER TABLE shopify_sync_runs
    ADD COLUMN lease_owner TEXT,
    ADD COLUMN lease_expires_at TIMESTAMPTZ,
    ADD COLUMN heartbeat_at TIMESTAMPTZ,
    ADD CONSTRAINT shopify_sync_runs_store_tenant_fk
    FOREIGN KEY (organization_id, store_id)
    REFERENCES shopify_stores(organization_id, id) ON DELETE CASCADE;

CREATE INDEX shopify_sync_runs_expired_lease_idx
    ON shopify_sync_runs (lease_expires_at)
    WHERE status = 'running';

ALTER TABLE shopify_products
    ADD CONSTRAINT shopify_products_org_store_id_unique UNIQUE (organization_id, store_id, id),
    ADD CONSTRAINT shopify_products_store_tenant_fk
    FOREIGN KEY (organization_id, store_id)
    REFERENCES shopify_stores(organization_id, id) ON DELETE CASCADE;

ALTER TABLE shopify_variants
    ADD CONSTRAINT shopify_variants_store_tenant_fk
    FOREIGN KEY (organization_id, store_id)
    REFERENCES shopify_stores(organization_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT shopify_variants_product_tenant_fk
    FOREIGN KEY (organization_id, store_id, product_id)
    REFERENCES shopify_products(organization_id, store_id, id) ON DELETE CASCADE;

ALTER TABLE shopify_collections
    ADD CONSTRAINT shopify_collections_store_tenant_fk
    FOREIGN KEY (organization_id, store_id)
    REFERENCES shopify_stores(organization_id, id) ON DELETE CASCADE;

ALTER TABLE shopify_themes
    ADD CONSTRAINT shopify_themes_store_tenant_fk
    FOREIGN KEY (organization_id, store_id)
    REFERENCES shopify_stores(organization_id, id) ON DELETE CASCADE;

ALTER TABLE shopify_resource_snapshots
    ADD CONSTRAINT shopify_snapshots_store_tenant_fk
    FOREIGN KEY (organization_id, store_id)
    REFERENCES shopify_stores(organization_id, id) ON DELETE CASCADE;

ALTER TABLE webhook_events
    ADD CONSTRAINT webhook_events_store_tenant_fk
    FOREIGN KEY (organization_id, store_id)
    REFERENCES shopify_stores(organization_id, id) ON DELETE CASCADE;

ALTER TABLE shopify_themes DROP CONSTRAINT shopify_themes_role_check;
ALTER TABLE shopify_themes ADD CONSTRAINT shopify_themes_role_check
    CHECK (role IN ('ARCHIVED', 'DEMO', 'DEVELOPMENT', 'LOCKED', 'MAIN', 'UNPUBLISHED', 'MOBILE'));
