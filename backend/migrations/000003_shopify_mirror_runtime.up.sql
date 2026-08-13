CREATE TABLE shopify_sync_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    mode TEXT NOT NULL DEFAULT 'full' CHECK (mode IN ('full', 'incremental')),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    job_id TEXT NOT NULL UNIQUE,
    products_count INTEGER NOT NULL DEFAULT 0 CHECK (products_count >= 0),
    variants_count INTEGER NOT NULL DEFAULT 0 CHECK (variants_count >= 0),
    collections_count INTEGER NOT NULL DEFAULT 0 CHECK (collections_count >= 0),
    themes_count INTEGER NOT NULL DEFAULT 0 CHECK (themes_count >= 0),
    error_code TEXT,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX shopify_sync_runs_store_created_idx
    ON shopify_sync_runs (organization_id, store_id, created_at DESC);

CREATE UNIQUE INDEX shopify_sync_runs_one_active_idx
    ON shopify_sync_runs (organization_id, store_id)
    WHERE status IN ('queued', 'running');

CREATE TABLE shopify_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    shopify_gid TEXT NOT NULL,
    handle TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    vendor TEXT NOT NULL DEFAULT '',
    product_type TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, store_id, shopify_gid)
);

CREATE INDEX shopify_products_store_status_idx
    ON shopify_products (organization_id, store_id, status);

CREATE TABLE shopify_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES shopify_products(id) ON DELETE CASCADE,
    shopify_gid TEXT NOT NULL,
    sku TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    price NUMERIC(20, 4),
    compare_at_price NUMERIC(20, 4),
    inventory_quantity INTEGER,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, store_id, shopify_gid)
);

CREATE INDEX shopify_variants_product_idx ON shopify_variants (product_id);
CREATE INDEX shopify_variants_store_sku_idx
    ON shopify_variants (organization_id, store_id, sku)
    WHERE sku <> '';

CREATE TABLE shopify_collections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    shopify_gid TEXT NOT NULL,
    handle TEXT NOT NULL,
    title TEXT NOT NULL,
    collection_type TEXT NOT NULL DEFAULT '',
    products_count INTEGER NOT NULL DEFAULT 0 CHECK (products_count >= 0),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, store_id, shopify_gid)
);

CREATE INDEX shopify_collections_store_handle_idx
    ON shopify_collections (organization_id, store_id, handle);

CREATE TABLE shopify_themes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    shopify_gid TEXT NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('MAIN', 'UNPUBLISHED', 'DEVELOPMENT', 'DEMO', 'MOBILE')),
    processing BOOLEAN NOT NULL DEFAULT false,
    processing_failed BOOLEAN NOT NULL DEFAULT false,
    theme_store_id BIGINT,
    source_release_id UUID,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, store_id, shopify_gid)
);

CREATE INDEX shopify_themes_store_role_idx
    ON shopify_themes (organization_id, store_id, role);

CREATE TABLE shopify_resource_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    store_id UUID NOT NULL REFERENCES shopify_stores(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL,
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_updated_at TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, store_id, resource_type)
);

ALTER TABLE webhook_events
    ADD COLUMN IF NOT EXISTS store_id UUID REFERENCES shopify_stores(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS payload_object_key TEXT,
    ADD COLUMN IF NOT EXISTS processing_status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS error_code TEXT,
    ADD COLUMN IF NOT EXISTS error_message TEXT;

CREATE TABLE outbox_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_key TEXT NOT NULL UNIQUE,
    envelope JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'publishing', 'published', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX outbox_messages_pending_idx
    ON outbox_messages (status, available_at, created_at)
    WHERE status IN ('pending', 'failed');

INSERT INTO permissions (code, description) VALUES
    ('shopify:sync', 'Synchronize Shopify store resources')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

INSERT INTO role_permissions(role_id, permission_code)
SELECT id, 'shopify:sync' FROM roles WHERE name = 'Operator'
ON CONFLICT DO NOTHING;
