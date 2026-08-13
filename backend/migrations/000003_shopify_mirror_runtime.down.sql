DROP TABLE IF EXISTS outbox_messages;

ALTER TABLE webhook_events
    DROP COLUMN IF EXISTS error_message,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS processing_status,
    DROP COLUMN IF EXISTS payload_object_key,
    DROP COLUMN IF EXISTS store_id;

DROP TABLE IF EXISTS shopify_resource_snapshots;
DROP TABLE IF EXISTS shopify_themes;
DROP TABLE IF EXISTS shopify_collections;
DROP TABLE IF EXISTS shopify_variants;
DROP TABLE IF EXISTS shopify_products;
DROP TABLE IF EXISTS shopify_sync_runs;
DELETE FROM role_permissions WHERE permission_code = 'shopify:sync';
DELETE FROM permissions WHERE code = 'shopify:sync';
