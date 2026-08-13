package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xg-management/platform/backend/internal/admin"
	"github.com/xg-management/platform/backend/internal/auth"
)

func (c *Client) ResolvePrincipal(ctx context.Context, userID, organizationID string) (auth.Principal, error) {
	var principal auth.Principal
	err := c.pool.QueryRow(ctx, `
		SELECT u.id::text, ou.organization_id::text, u.display_name
		FROM organization_users ou
		JOIN users u ON u.id = ou.user_id
		WHERE ou.user_id = $1 AND ou.organization_id = $2 AND u.status = 'active'`, userID, organizationID).
		Scan(&principal.UserID, &principal.OrganizationID, &principal.DisplayName)
	if err != nil {
		return auth.Principal{}, fmt.Errorf("resolve user principal: %w", err)
	}
	rows, err := c.pool.Query(ctx, `
		SELECT DISTINCT rp.permission_code
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		WHERE ur.organization_id = $1 AND ur.user_id = $2
		ORDER BY rp.permission_code`, organizationID, userID)
	if err != nil {
		return auth.Principal{}, fmt.Errorf("resolve principal permissions: %w", err)
	}
	defer rows.Close()
	principal.Permissions = []string{}
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return auth.Principal{}, err
		}
		principal.Permissions = append(principal.Permissions, permission)
	}
	return principal, rows.Err()
}

func (c *Client) OrganizationIDBySlug(ctx context.Context, slug string) (string, error) {
	var id string
	err := c.pool.QueryRow(ctx, `SELECT id::text FROM organizations WHERE slug = $1`, strings.ToLower(strings.TrimSpace(slug))).Scan(&id)
	return id, err
}

func (c *Client) ListUsers(ctx context.Context, organizationID string) ([]admin.User, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT u.id::text, COALESCE(u.email, ''), u.display_name, u.status, u.last_login_at, u.created_at,
		       COALESCE((SELECT jsonb_agg(jsonb_build_object('id', r.id::text, 'name', r.name) ORDER BY r.name)
		                 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		                 WHERE ur.organization_id = ou.organization_id AND ur.user_id = u.id), '[]'::jsonb)
		FROM organization_users ou JOIN users u ON u.id = ou.user_id
		WHERE ou.organization_id = $1 ORDER BY u.created_at, u.display_name`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	result := []admin.User{}
	for rows.Next() {
		var user admin.User
		var roles []byte
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.LastLoginAt, &user.CreatedAt, &roles); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if err := json.Unmarshal(roles, &user.Roles); err != nil {
			return nil, fmt.Errorf("decode user roles: %w", err)
		}
		result = append(result, user)
	}
	return result, rows.Err()
}

func (c *Client) CreateUser(ctx context.Context, organizationID, displayName, email string) (admin.User, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return admin.User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var user admin.User
	err = tx.QueryRow(ctx, `INSERT INTO users (display_name, email) VALUES ($1, NULLIF($2, ''))
		RETURNING id::text, COALESCE(email, ''), display_name, status, last_login_at, created_at`, displayName, email).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.LastLoginAt, &user.CreatedAt)
	if err != nil {
		return admin.User{}, fmt.Errorf("create user: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO organization_users (organization_id, user_id) VALUES ($1, $2)`, organizationID, user.ID); err != nil {
		return admin.User{}, fmt.Errorf("join user to organization: %w", err)
	}
	user.Roles = []admin.RoleRef{}
	if err := tx.Commit(ctx); err != nil {
		return admin.User{}, err
	}
	return user, nil
}

func (c *Client) UpdateUser(ctx context.Context, organizationID, userID, displayName, email, status string) (admin.User, error) {
	var user admin.User
	err := c.pool.QueryRow(ctx, `
		UPDATE users u SET display_name = $3, email = NULLIF($4, ''), status = $5, updated_at = now()
		FROM organization_users ou WHERE ou.user_id = u.id AND ou.organization_id = $1 AND u.id = $2
		RETURNING u.id::text, COALESCE(u.email, ''), u.display_name, u.status, u.last_login_at, u.created_at`,
		organizationID, userID, displayName, email, status).
		Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.LastLoginAt, &user.CreatedAt)
	user.Roles = []admin.RoleRef{}
	return user, err
}

func (c *Client) SetUserRoles(ctx context.Context, organizationID, userID string, roleIDs []string) error {
	return c.withTransaction(ctx, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_users WHERE organization_id=$1 AND user_id=$2)`, organizationID, userID).Scan(&exists); err != nil || !exists {
			return fmt.Errorf("user does not belong to organization")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE organization_id=$1 AND user_id=$2`, organizationID, userID); err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			command, err := tx.Exec(ctx, `INSERT INTO user_roles (organization_id, user_id, role_id)
				SELECT $1, $2, id FROM roles WHERE id=$3 AND organization_id=$1`, organizationID, userID, roleID)
			if err != nil || command.RowsAffected() != 1 {
				return fmt.Errorf("role %s does not belong to organization", roleID)
			}
		}
		return nil
	})
}

func (c *Client) ListRoles(ctx context.Context, organizationID string) ([]admin.Role, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT r.id::text, r.name, r.description, r.is_system, r.created_at,
		       ARRAY(SELECT permission_code FROM role_permissions WHERE role_id=r.id ORDER BY permission_code),
		       ARRAY(SELECT menu_id::text FROM role_menus WHERE role_id=r.id ORDER BY menu_id::text)
		FROM roles r WHERE r.organization_id=$1 ORDER BY r.is_system DESC, r.name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []admin.Role{}
	for rows.Next() {
		var role admin.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt, &role.Permissions, &role.MenuIDs); err != nil {
			return nil, err
		}
		result = append(result, role)
	}
	return result, rows.Err()
}

func (c *Client) CreateRole(ctx context.Context, organizationID, name, description string) (admin.Role, error) {
	var role admin.Role
	err := c.pool.QueryRow(ctx, `INSERT INTO roles (organization_id,name,description) VALUES ($1,$2,$3)
		RETURNING id::text,name,description,is_system,created_at`, organizationID, name, description).
		Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt)
	role.Permissions = []string{}
	role.MenuIDs = []string{}
	return role, err
}

func (c *Client) UpdateRole(ctx context.Context, organizationID, roleID, name, description string) (admin.Role, error) {
	var role admin.Role
	err := c.pool.QueryRow(ctx, `UPDATE roles SET name=$3,description=$4,updated_at=now()
		WHERE organization_id=$1 AND id=$2 RETURNING id::text,name,description,is_system,created_at`, organizationID, roleID, name, description).
		Scan(&role.ID, &role.Name, &role.Description, &role.IsSystem, &role.CreatedAt)
	role.Permissions = []string{}
	role.MenuIDs = []string{}
	return role, err
}

func (c *Client) DeleteRole(ctx context.Context, organizationID, roleID string) error {
	command, err := c.pool.Exec(ctx, `DELETE FROM roles WHERE organization_id=$1 AND id=$2 AND is_system=false`, organizationID, roleID)
	if err == nil && command.RowsAffected() != 1 {
		return fmt.Errorf("role not found or is a system role")
	}
	return err
}

func (c *Client) SetRolePermissions(ctx context.Context, organizationID, roleID string, permissions []string) error {
	return c.replaceRoleValues(ctx, organizationID, roleID, permissions,
		`DELETE FROM role_permissions WHERE role_id=$1`,
		`INSERT INTO role_permissions(role_id,permission_code) SELECT $1,code FROM permissions WHERE code=$2`)
}

func (c *Client) SetRoleMenus(ctx context.Context, organizationID, roleID string, menuIDs []string) error {
	return c.replaceRoleValues(ctx, organizationID, roleID, menuIDs,
		`DELETE FROM role_menus WHERE role_id=$1`,
		`INSERT INTO role_menus(role_id,menu_id) SELECT $1,id FROM menus WHERE id=$2 AND organization_id=$3`)
}

func (c *Client) replaceRoleValues(ctx context.Context, organizationID, roleID string, values []string, deleteSQL, insertSQL string) error {
	return c.withTransaction(ctx, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roles WHERE organization_id=$1 AND id=$2)`, organizationID, roleID).Scan(&exists); err != nil || !exists {
			return fmt.Errorf("role not found")
		}
		if _, err := tx.Exec(ctx, deleteSQL, roleID); err != nil {
			return err
		}
		for _, value := range values {
			var command pgconn.CommandTag
			var err error
			if strings.Contains(insertSQL, "$3") {
				command, err = tx.Exec(ctx, insertSQL, roleID, value, organizationID)
			} else {
				command, err = tx.Exec(ctx, insertSQL, roleID, value)
			}
			if err != nil || command.RowsAffected() != 1 {
				return fmt.Errorf("invalid assignment %s", value)
			}
		}
		return nil
	})
}

func (c *Client) ListPermissions(ctx context.Context) ([]admin.Permission, error) {
	rows, err := c.pool.Query(ctx, `SELECT code,description FROM permissions ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []admin.Permission{}
	for rows.Next() {
		var item admin.Permission
		if err := rows.Scan(&item.Code, &item.Description); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (c *Client) ListMenus(ctx context.Context, organizationID string) ([]admin.Menu, error) {
	return c.queryMenus(ctx, `SELECT id::text,parent_id::text,code,name,path,icon,sort_order,COALESCE(required_permission,''),status FROM menus WHERE organization_id=$1 ORDER BY sort_order,created_at`, organizationID)
}

func (c *Client) ListUserMenus(ctx context.Context, organizationID, userID string) ([]admin.Menu, error) {
	return c.queryMenus(ctx, `SELECT DISTINCT m.id::text,m.parent_id::text,m.code,m.name,m.path,m.icon,m.sort_order,COALESCE(m.required_permission,''),m.status
		FROM menus m JOIN role_menus rm ON rm.menu_id=m.id JOIN user_roles ur ON ur.role_id=rm.role_id
		WHERE m.organization_id=$1 AND ur.organization_id=$1 AND ur.user_id=$2 AND m.status='active' ORDER BY m.sort_order,m.id::text`, organizationID, userID)
}

func (c *Client) queryMenus(ctx context.Context, query string, args ...any) ([]admin.Menu, error) {
	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []admin.Menu{}
	for rows.Next() {
		var item admin.Menu
		if err := rows.Scan(&item.ID, &item.ParentID, &item.Code, &item.Name, &item.Path, &item.Icon, &item.SortOrder, &item.RequiredPermission, &item.Status); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (c *Client) CreateMenu(ctx context.Context, organizationID string, item admin.Menu) (admin.Menu, error) {
	err := c.pool.QueryRow(ctx, `INSERT INTO menus(organization_id,parent_id,code,name,path,icon,sort_order,required_permission,status)
		VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5,$6,$7,NULLIF($8,''),$9)
		RETURNING id::text,parent_id::text,code,name,path,icon,sort_order,COALESCE(required_permission,''),status`, organizationID, stringValue(item.ParentID), item.Code, item.Name, item.Path, item.Icon, item.SortOrder, item.RequiredPermission, item.Status).
		Scan(&item.ID, &item.ParentID, &item.Code, &item.Name, &item.Path, &item.Icon, &item.SortOrder, &item.RequiredPermission, &item.Status)
	return item, err
}

func (c *Client) UpdateMenu(ctx context.Context, organizationID string, item admin.Menu) (admin.Menu, error) {
	err := c.pool.QueryRow(ctx, `UPDATE menus SET parent_id=NULLIF($3,'')::uuid,code=$4,name=$5,path=$6,icon=$7,sort_order=$8,required_permission=NULLIF($9,''),status=$10,updated_at=now()
		WHERE organization_id=$1 AND id=$2 RETURNING id::text,parent_id::text,code,name,path,icon,sort_order,COALESCE(required_permission,''),status`, organizationID, item.ID, stringValue(item.ParentID), item.Code, item.Name, item.Path, item.Icon, item.SortOrder, item.RequiredPermission, item.Status).
		Scan(&item.ID, &item.ParentID, &item.Code, &item.Name, &item.Path, &item.Icon, &item.SortOrder, &item.RequiredPermission, &item.Status)
	return item, err
}

func (c *Client) DeleteMenu(ctx context.Context, organizationID, menuID string) error {
	command, err := c.pool.Exec(ctx, `DELETE FROM menus WHERE organization_id=$1 AND id=$2`, organizationID, menuID)
	if err == nil && command.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return err
}

func (c *Client) ListSettings(ctx context.Context, organizationID string) ([]admin.Setting, error) {
	rows, err := c.pool.Query(ctx, `SELECT id::text,namespace,key,value,description,updated_at FROM system_settings WHERE organization_id=$1 ORDER BY namespace,key`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []admin.Setting{}
	for rows.Next() {
		var item admin.Setting
		if err := rows.Scan(&item.ID, &item.Namespace, &item.Key, &item.Value, &item.Description, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (c *Client) UpsertSetting(ctx context.Context, organizationID, userID string, item admin.Setting) (admin.Setting, error) {
	err := c.pool.QueryRow(ctx, `INSERT INTO system_settings(organization_id,namespace,key,value,description,updated_by) VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(organization_id,namespace,key) DO UPDATE SET value=EXCLUDED.value,description=EXCLUDED.description,updated_by=EXCLUDED.updated_by,updated_at=now()
		RETURNING id::text,namespace,key,value,description,updated_at`, organizationID, item.Namespace, item.Key, item.Value, item.Description, userID).
		Scan(&item.ID, &item.Namespace, &item.Key, &item.Value, &item.Description, &item.UpdatedAt)
	return item, err
}

func (c *Client) DeleteSetting(ctx context.Context, organizationID, namespace, key string) error {
	command, err := c.pool.Exec(ctx, `DELETE FROM system_settings WHERE organization_id=$1 AND namespace=$2 AND key=$3`, organizationID, namespace, key)
	if err == nil && command.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return err
}

func (c *Client) GetIntegrationConfig(ctx context.Context, organizationID, provider string) (admin.IntegrationConfig, error) {
	var item admin.IntegrationConfig
	item.Provider = provider
	err := c.pool.QueryRow(ctx, `SELECT provider,public_config,encrypted_secrets,enabled,updated_at FROM integration_configs WHERE organization_id=$1 AND provider=$2`, organizationID, provider).
		Scan(&item.Provider, &item.PublicConfig, &item.EncryptedSecrets, &item.Enabled, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		item.PublicConfig = json.RawMessage(`{}`)
		return item, nil
	}
	return item, err
}

func (c *Client) UpsertIntegrationConfig(ctx context.Context, organizationID, provider string, public json.RawMessage, secrets []byte, enabled bool, userID string) (admin.IntegrationConfig, error) {
	var item admin.IntegrationConfig
	err := c.pool.QueryRow(ctx, `INSERT INTO integration_configs(organization_id,provider,public_config,encrypted_secrets,encryption_key_id,enabled,updated_by)
		VALUES($1,$2,$3,$4,'primary',$5,$6) ON CONFLICT(organization_id,provider) DO UPDATE SET public_config=EXCLUDED.public_config,
		encrypted_secrets=COALESCE(EXCLUDED.encrypted_secrets,integration_configs.encrypted_secrets),enabled=EXCLUDED.enabled,updated_by=EXCLUDED.updated_by,updated_at=now()
		RETURNING provider,public_config,encrypted_secrets,enabled,updated_at`, organizationID, provider, public, secrets, enabled, userID).
		Scan(&item.Provider, &item.PublicConfig, &item.EncryptedSecrets, &item.Enabled, &item.UpdatedAt)
	return item, err
}

func (c *Client) ListDingTalkUsers(ctx context.Context, organizationID string) ([]admin.DingTalkUser, error) {
	rows, err := c.pool.Query(ctx, `SELECT u.id::text,u.display_name,COALESCE(u.email,''),u.status,i.provider_user_id,i.metadata,u.last_login_at
		FROM user_identities i JOIN users u ON u.id=i.user_id WHERE i.organization_id=$1 AND i.provider='dingtalk' ORDER BY u.display_name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []admin.DingTalkUser{}
	for rows.Next() {
		var item admin.DingTalkUser
		if err := rows.Scan(&item.UserID, &item.DisplayName, &item.Email, &item.Status, &item.ProviderUserID, &item.Metadata, &item.LastLoginAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (c *Client) UpsertDingTalkIdentity(ctx context.Context, input admin.DingTalkIdentityInput) (string, error) {
	var userID string
	err := c.pool.QueryRow(ctx, `SELECT user_id::text FROM user_identities WHERE organization_id=$1 AND provider='dingtalk' AND provider_user_id=$2`, input.OrganizationID, input.ProviderUserID).Scan(&userID)
	if err == nil {
		_, err = c.pool.Exec(ctx, `UPDATE users SET display_name=$2,email=COALESCE(NULLIF($3,''),email),last_login_at=now(),updated_at=now() WHERE id=$1`, userID, input.DisplayName, input.Email)
		return userID, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if input.UserID != "" {
		err = c.withTransaction(ctx, func(tx pgx.Tx) error {
			var belongs bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_users WHERE organization_id=$1 AND user_id=$2)`, input.OrganizationID, input.UserID).Scan(&belongs); err != nil || !belongs {
				return fmt.Errorf("user does not belong to organization")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO user_identities(organization_id,user_id,provider,provider_user_id,metadata) VALUES($1,$2,'dingtalk',$3,$4)`, input.OrganizationID, input.UserID, input.ProviderUserID, input.Metadata); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE users SET display_name=$2,email=COALESCE(NULLIF($3,''),email),last_login_at=now(),updated_at=now() WHERE id=$1`, input.UserID, input.DisplayName, input.Email)
			return err
		})
		return input.UserID, err
	}
	err = c.withTransaction(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO users(display_name,email,last_login_at) VALUES($1,NULLIF($2,''),now()) RETURNING id::text`, input.DisplayName, input.Email).Scan(&userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO organization_users(organization_id,user_id) VALUES($1,$2)`, input.OrganizationID, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_identities(organization_id,user_id,provider,provider_user_id,metadata) VALUES($1,$2,'dingtalk',$3,$4)`, input.OrganizationID, userID, input.ProviderUserID, input.Metadata); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO user_roles(organization_id,user_id,role_id) SELECT $1,$2,id FROM roles WHERE organization_id=$1 AND name='Viewer'`, input.OrganizationID, userID)
		return err
	})
	return userID, err
}

func (c *Client) GetStore(ctx context.Context, organizationID, storeID string) (admin.StoreDetails, error) {
	var item admin.StoreDetails
	err := c.pool.QueryRow(ctx, `SELECT s.id::text,s.name,s.shop_domain,COALESCE(s.primary_domain,''),s.status,COALESCE(s.currency::text,''),COALESCE(s.timezone,''),COALESCE(s.plan_name,''),
		COALESCE(a.scopes,'{}'),a.expires_at,s.last_synced_at,COALESCE(a.last_error,''),s.created_at FROM shopify_stores s LEFT JOIN integration_accounts a ON a.id=s.integration_account_id
		WHERE s.organization_id=$1 AND s.id=$2`, organizationID, storeID).Scan(&item.ID, &item.Name, &item.Domain, &item.PrimaryDomain, &item.Status, &item.Currency, &item.Timezone, &item.PlanName, &item.Scopes, &item.ExpiresAt, &item.LastSyncAt, &item.LastError, &item.CreatedAt)
	return item, err
}

func (c *Client) UpdateStore(ctx context.Context, organizationID, storeID, name, status string) (admin.StoreDetails, error) {
	_, err := c.pool.Exec(ctx, `UPDATE shopify_stores SET name=$3,status=$4,updated_at=now() WHERE organization_id=$1 AND id=$2`, organizationID, storeID, name, status)
	if err != nil {
		return admin.StoreDetails{}, err
	}
	return c.GetStore(ctx, organizationID, storeID)
}

func (c *Client) DisconnectStore(ctx context.Context, organizationID, storeID string) error {
	return c.withTransaction(ctx, func(tx pgx.Tx) error {
		var accountID *string
		if err := tx.QueryRow(ctx, `UPDATE shopify_stores SET status='disconnected',updated_at=now() WHERE organization_id=$1 AND id=$2 RETURNING integration_account_id::text`, organizationID, storeID).Scan(&accountID); err != nil {
			return err
		}
		if accountID != nil {
			_, err := tx.Exec(ctx, `UPDATE integration_accounts SET status='disconnected',encrypted_credentials=NULL,updated_at=now() WHERE organization_id=$1 AND id=$2`, organizationID, *accountID)
			return err
		}
		return nil
	})
}

func (c *Client) UpsertShopifyAuthorization(ctx context.Context, input admin.ShopifyAuthorization) (admin.StoreDetails, error) {
	var storeID string
	err := c.withTransaction(ctx, func(tx pgx.Tx) error {
		var accountID string
		if err := tx.QueryRow(ctx, `INSERT INTO integration_accounts(organization_id,provider,external_account_id,display_name,status,encrypted_credentials,encryption_key_id,scopes,expires_at,refresh_expires_at,last_error)
			VALUES($1,'shopify',$2,$3,'connected',$4,'primary',$5,$6,$7,NULL) ON CONFLICT(organization_id,provider,external_account_id) DO UPDATE SET display_name=EXCLUDED.display_name,status='connected',encrypted_credentials=EXCLUDED.encrypted_credentials,scopes=EXCLUDED.scopes,expires_at=EXCLUDED.expires_at,refresh_expires_at=EXCLUDED.refresh_expires_at,last_error=NULL,updated_at=now() RETURNING id::text`, input.OrganizationID, input.Domain, input.DisplayName, input.EncryptedTokens, input.Scopes, input.ExpiresAt, input.RefreshExpiresAt).Scan(&accountID); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO shopify_stores(organization_id,integration_account_id,name,shop_domain,status,currency,timezone,shopify_gid,plan_name,primary_domain)
			VALUES($1,$2,$3,$4,'connected',NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,'')) ON CONFLICT(organization_id,shop_domain) DO UPDATE SET integration_account_id=EXCLUDED.integration_account_id,name=EXCLUDED.name,status='connected',currency=EXCLUDED.currency,timezone=EXCLUDED.timezone,shopify_gid=EXCLUDED.shopify_gid,plan_name=EXCLUDED.plan_name,primary_domain=EXCLUDED.primary_domain,updated_at=now() RETURNING id::text`, input.OrganizationID, accountID, input.DisplayName, input.Domain, input.Currency, input.Timezone, input.ShopifyGID, input.PlanName, input.PrimaryDomain).Scan(&storeID)
	})
	if err != nil {
		return admin.StoreDetails{}, err
	}
	return c.GetStore(ctx, input.OrganizationID, storeID)
}

func (c *Client) withTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ = time.Time{}
