DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS role_menus;
DROP TABLE IF EXISTS menus;
DROP TABLE IF EXISTS organization_users;
DROP TABLE IF EXISTS integration_configs;

ALTER TABLE shopify_stores DROP COLUMN IF EXISTS primary_domain;
ALTER TABLE shopify_stores DROP COLUMN IF EXISTS plan_name;
ALTER TABLE shopify_stores DROP COLUMN IF EXISTS shopify_gid;
ALTER TABLE integration_accounts DROP COLUMN IF EXISTS last_error;
ALTER TABLE integration_accounts DROP COLUMN IF EXISTS refresh_expires_at;
ALTER TABLE integration_accounts DROP COLUMN IF EXISTS expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;

DELETE FROM permissions WHERE code IN ('*', 'settings:manage', 'menus:manage');
