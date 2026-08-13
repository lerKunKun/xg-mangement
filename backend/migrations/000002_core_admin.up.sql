ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
ALTER TABLE integration_accounts ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE integration_accounts ADD COLUMN IF NOT EXISTS refresh_expires_at TIMESTAMPTZ;
ALTER TABLE integration_accounts ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE shopify_stores ADD COLUMN IF NOT EXISTS shopify_gid TEXT;
ALTER TABLE shopify_stores ADD COLUMN IF NOT EXISTS plan_name TEXT;
ALTER TABLE shopify_stores ADD COLUMN IF NOT EXISTS primary_domain TEXT;

INSERT INTO permissions (code, description) VALUES
    ('*', 'Full organization access'),
    ('settings:manage', 'Manage organization settings'),
    ('menus:manage', 'Manage organization navigation')
ON CONFLICT (code) DO UPDATE SET description = EXCLUDED.description;

CREATE TABLE integration_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    public_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    encrypted_secrets BYTEA,
    encryption_key_id TEXT,
    enabled BOOLEAN NOT NULL DEFAULT false,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, provider)
);

CREATE TABLE organization_users (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE menus (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES menus(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    icon TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    required_permission TEXT REFERENCES permissions(code) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'hidden')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, code)
);

CREATE INDEX menus_org_parent_sort_idx ON menus (organization_id, parent_id, sort_order);

CREATE TABLE role_menus (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    menu_id UUID NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, menu_id)
);

CREATE TABLE system_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    namespace TEXT NOT NULL DEFAULT 'general',
    key TEXT NOT NULL,
    value JSONB NOT NULL DEFAULT 'null'::jsonb,
    description TEXT NOT NULL DEFAULT '',
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, namespace, key)
);

INSERT INTO organizations (id, name, slug) VALUES
    ('00000000-0000-0000-0000-000000000001', 'XG Local', 'local')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, slug = EXCLUDED.slug, updated_at = now();

INSERT INTO users (id, email, display_name, status) VALUES
    ('00000000-0000-0000-0000-000000000002', 'owner@local.xg', 'Local Owner', 'active')
ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, status = 'active', updated_at = now();

INSERT INTO user_identities (organization_id, user_id, provider, provider_user_id, metadata) VALUES
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'dev', 'local-owner', '{"seeded":true}'::jsonb)
ON CONFLICT (organization_id, provider, provider_user_id) DO NOTHING;

INSERT INTO organization_users (organization_id, user_id) VALUES
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002')
ON CONFLICT DO NOTHING;

INSERT INTO roles (id, organization_id, name, description, is_system) VALUES
    ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', 'Owner', 'Organization owner with full access', true),
    ('00000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'Operator', 'Operates stores, assets, reports and approvals', true),
    ('00000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'Viewer', 'Read-only business access', true)
ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description, is_system = true, updated_at = now();

INSERT INTO role_permissions (role_id, permission_code) VALUES
    ('00000000-0000-0000-0000-000000000010', '*'),
    ('00000000-0000-0000-0000-000000000011', 'stores:read'),
    ('00000000-0000-0000-0000-000000000011', 'stores:write'),
    ('00000000-0000-0000-0000-000000000011', 'assets:read'),
    ('00000000-0000-0000-0000-000000000011', 'assets:write'),
    ('00000000-0000-0000-0000-000000000011', 'approvals:read'),
    ('00000000-0000-0000-0000-000000000011', 'approvals:request'),
    ('00000000-0000-0000-0000-000000000011', 'integrations:read'),
    ('00000000-0000-0000-0000-000000000011', 'reports:read'),
    ('00000000-0000-0000-0000-000000000012', 'stores:read'),
    ('00000000-0000-0000-0000-000000000012', 'assets:read'),
    ('00000000-0000-0000-0000-000000000012', 'approvals:read'),
    ('00000000-0000-0000-0000-000000000012', 'integrations:read'),
    ('00000000-0000-0000-0000-000000000012', 'reports:read')
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (organization_id, user_id, role_id) VALUES
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000010')
ON CONFLICT DO NOTHING;

INSERT INTO menus (id, organization_id, parent_id, code, name, path, icon, sort_order, required_permission) VALUES
    ('00000000-0000-0000-0000-000000000100', '00000000-0000-0000-0000-000000000001', NULL, 'dashboard', '工作台', '/dashboard', 'LayoutDashboard', 10, 'reports:read'),
    ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000001', NULL, 'stores', 'Shopify 店铺', '/stores', 'Store', 20, 'stores:read'),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000001', NULL, 'integrations', '平台集成', '', 'Cable', 30, 'integrations:read'),
    ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000102', 'integrations.dingtalk', '钉钉', '/integrations/dingtalk', 'MessageSquare', 31, 'integrations:read'),
    ('00000000-0000-0000-0000-000000000104', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000102', 'integrations.shopify', 'Shopify', '/integrations/shopify', 'ShoppingBag', 32, 'integrations:read'),
    ('00000000-0000-0000-0000-000000000105', '00000000-0000-0000-0000-000000000001', NULL, 'system', '系统管理', '', 'Settings2', 40, 'rbac:manage'),
    ('00000000-0000-0000-0000-000000000106', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000105', 'system.users', '用户管理', '/system/users', 'Users', 41, 'rbac:manage'),
    ('00000000-0000-0000-0000-000000000107', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000105', 'system.roles', '角色权限', '/system/roles', 'ShieldCheck', 42, 'rbac:manage'),
    ('00000000-0000-0000-0000-000000000108', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000105', 'system.menus', '菜单管理', '/system/menus', 'PanelLeft', 43, 'menus:manage'),
    ('00000000-0000-0000-0000-000000000109', '00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000105', 'system.settings', '系统配置', '/system/settings', 'SlidersHorizontal', 44, 'settings:manage')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, path = EXCLUDED.path, icon = EXCLUDED.icon, sort_order = EXCLUDED.sort_order, required_permission = EXCLUDED.required_permission, updated_at = now();

INSERT INTO role_menus (role_id, menu_id)
SELECT '00000000-0000-0000-0000-000000000010'::uuid, id FROM menus
WHERE organization_id = '00000000-0000-0000-0000-000000000001'
ON CONFLICT DO NOTHING;

INSERT INTO role_menus (role_id, menu_id)
SELECT '00000000-0000-0000-0000-000000000011'::uuid, id FROM menus
WHERE organization_id = '00000000-0000-0000-0000-000000000001' AND code IN ('dashboard', 'stores', 'integrations', 'integrations.dingtalk', 'integrations.shopify')
ON CONFLICT DO NOTHING;

INSERT INTO role_menus (role_id, menu_id)
SELECT '00000000-0000-0000-0000-000000000012'::uuid, id FROM menus
WHERE organization_id = '00000000-0000-0000-0000-000000000001' AND code IN ('dashboard', 'stores', 'integrations', 'integrations.dingtalk', 'integrations.shopify')
ON CONFLICT DO NOTHING;

INSERT INTO system_settings (organization_id, namespace, key, value, description, updated_by) VALUES
    ('00000000-0000-0000-0000-000000000001', 'general', 'site_name', '"XG Commerce OS"'::jsonb, 'Displayed product name', '00000000-0000-0000-0000-000000000002'),
    ('00000000-0000-0000-0000-000000000001', 'general', 'locale', '"zh-CN"'::jsonb, 'Default interface locale', '00000000-0000-0000-0000-000000000002'),
    ('00000000-0000-0000-0000-000000000001', 'shopify', 'default_market', '"CN"'::jsonb, 'Default market for new store operations', '00000000-0000-0000-0000-000000000002')
ON CONFLICT (organization_id, namespace, key) DO NOTHING;
