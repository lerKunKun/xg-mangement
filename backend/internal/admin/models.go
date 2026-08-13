package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xg-management/platform/backend/internal/auth"
)

type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email,omitempty"`
	DisplayName string     `json:"display_name"`
	Status      string     `json:"status"`
	Roles       []RoleRef  `json:"roles"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type RoleRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	MenuIDs     []string  `json:"menu_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

type Permission struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type Menu struct {
	ID                 string  `json:"id"`
	ParentID           *string `json:"parent_id,omitempty"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Path               string  `json:"path"`
	Icon               string  `json:"icon"`
	SortOrder          int     `json:"sort_order"`
	RequiredPermission string  `json:"required_permission,omitempty"`
	Status             string  `json:"status"`
}

type Setting struct {
	ID          string          `json:"id"`
	Namespace   string          `json:"namespace"`
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type IntegrationConfig struct {
	Provider         string          `json:"provider"`
	PublicConfig     json.RawMessage `json:"public_config"`
	EncryptedSecrets []byte          `json:"-"`
	Enabled          bool            `json:"enabled"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type DingTalkUser struct {
	UserID         string          `json:"user_id"`
	DisplayName    string          `json:"display_name"`
	Email          string          `json:"email,omitempty"`
	Status         string          `json:"status"`
	ProviderUserID string          `json:"provider_user_id"`
	Metadata       json.RawMessage `json:"metadata"`
	LastLoginAt    *time.Time      `json:"last_login_at,omitempty"`
}

type DingTalkIdentityInput struct {
	OrganizationID string
	UserID         string
	ProviderUserID string
	DisplayName    string
	Email          string
	Metadata       json.RawMessage
}

type ShopifyAuthorization struct {
	OrganizationID   string
	Domain           string
	DisplayName      string
	ShopifyGID       string
	PrimaryDomain    string
	Currency         string
	Timezone         string
	PlanName         string
	Scopes           []string
	EncryptedTokens  []byte
	ExpiresAt        *time.Time
	RefreshExpiresAt *time.Time
}

type StoreDetails struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Domain        string     `json:"domain"`
	PrimaryDomain string     `json:"primary_domain,omitempty"`
	Status        string     `json:"status"`
	Currency      string     `json:"currency,omitempty"`
	Timezone      string     `json:"timezone,omitempty"`
	PlanName      string     `json:"plan_name,omitempty"`
	Scopes        []string   `json:"scopes"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Repository interface {
	auth.PrincipalResolver
	OrganizationIDBySlug(context.Context, string) (string, error)
	ListUsers(context.Context, string) ([]User, error)
	CreateUser(context.Context, string, string, string) (User, error)
	UpdateUser(context.Context, string, string, string, string, string) (User, error)
	SetUserRoles(context.Context, string, string, []string) error
	ListRoles(context.Context, string) ([]Role, error)
	CreateRole(context.Context, string, string, string) (Role, error)
	UpdateRole(context.Context, string, string, string, string) (Role, error)
	DeleteRole(context.Context, string, string) error
	SetRolePermissions(context.Context, string, string, []string) error
	SetRoleMenus(context.Context, string, string, []string) error
	ListPermissions(context.Context) ([]Permission, error)
	ListMenus(context.Context, string) ([]Menu, error)
	ListUserMenus(context.Context, string, string) ([]Menu, error)
	CreateMenu(context.Context, string, Menu) (Menu, error)
	UpdateMenu(context.Context, string, Menu) (Menu, error)
	DeleteMenu(context.Context, string, string) error
	ListSettings(context.Context, string) ([]Setting, error)
	UpsertSetting(context.Context, string, string, Setting) (Setting, error)
	DeleteSetting(context.Context, string, string, string) error
	GetIntegrationConfig(context.Context, string, string) (IntegrationConfig, error)
	UpsertIntegrationConfig(context.Context, string, string, json.RawMessage, []byte, bool, string) (IntegrationConfig, error)
	ListDingTalkUsers(context.Context, string) ([]DingTalkUser, error)
	UpsertDingTalkIdentity(context.Context, DingTalkIdentityInput) (string, error)
	GetStore(context.Context, string, string) (StoreDetails, error)
	UpdateStore(context.Context, string, string, string, string) (StoreDetails, error)
	DisconnectStore(context.Context, string, string) error
	UpsertShopifyAuthorization(context.Context, ShopifyAuthorization) (StoreDetails, error)
}
