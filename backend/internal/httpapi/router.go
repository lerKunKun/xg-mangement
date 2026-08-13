package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/admin"
	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/integrations"
	"github.com/xg-management/platform/backend/internal/rbac"
)

type Authenticator interface {
	Authenticate(*http.Request) (auth.Principal, bool)
}

type Authorizer interface {
	Allowed(context.Context, auth.Principal, rbac.Permission) (bool, error)
}

type PolicyReloader interface {
	Reload(context.Context) error
}

type Store struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Status   string `json:"status"`
	LastSync string `json:"last_sync,omitempty"`
}

type Asset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
	Status      string `json:"status"`
}

type Approval struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Status             string `json:"status"`
	DingTalkInstanceID string `json:"dingtalk_instance_id,omitempty"`
}

type StoreRepository interface {
	List(context.Context, string) ([]Store, error)
}

type AssetRepository interface {
	List(context.Context, string) ([]Asset, error)
}

type ApprovalRepository interface {
	List(context.Context, string) ([]Approval, error)
}

type Dependencies struct {
	Authenticator   Authenticator
	Authorizer      Authorizer
	PolicyReloader  PolicyReloader
	Stores          StoreRepository
	Assets          AssetRepository
	Approvals       ApprovalRepository
	Integrations    []integrations.Status
	Admin           admin.Repository
	Sessions        *auth.SessionManager
	IntegrationFlow *IntegrationDependencies
	DevLoginEnabled bool
	SecureCookies   bool
	SessionTTL      time.Duration
	Webhooks        *ShopifyWebhookDependencies
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), requestIDMiddleware())

	router.GET("/healthz", func(c *gin.Context) {
		respondData(c, http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		respondData(c, http.StatusOK, gin.H{"status": "ready", "checked_at": time.Now().UTC()})
	})
	if dependencies.IntegrationFlow != nil {
		router.POST("/api/v1/auth/dev-login", devLogin(dependencies.Admin, dependencies.Sessions, dependencies.DevLoginEnabled, dependencies.SecureCookies, dependencies.SessionTTL))
		router.GET("/api/v1/auth/dingtalk/login", dingTalkLogin(*dependencies.IntegrationFlow, false))
		router.GET("/api/v1/integrations/dingtalk/callback", dingTalkCallback(*dependencies.IntegrationFlow))
		router.GET("/api/v1/integrations/shopify/callback", shopifyCallback(*dependencies.IntegrationFlow))
	} else {
		router.GET("/api/v1/integrations/dingtalk/callback", integrationBoundary(dependencies.Integrations, integrations.ProviderDingTalk))
		router.GET("/api/v1/integrations/shopify/callback", integrationBoundary(dependencies.Integrations, integrations.ProviderShopify))
	}
	if dependencies.Webhooks != nil {
		router.POST("/api/v1/webhooks/shopify", shopifyWebhook(*dependencies.Webhooks))
	} else {
		router.POST("/api/v1/webhooks/shopify", integrationBoundary(dependencies.Integrations, integrations.ProviderShopify))
	}

	api := router.Group("/api/v1")
	api.Use(authenticate(dependencies.Authenticator))
	api.GET("/me", func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		respondData(c, http.StatusOK, principal)
	})
	if dependencies.Sessions != nil {
		api.POST("/auth/logout", logout(dependencies.Sessions, dependencies.SecureCookies))
	}
	api.GET("/stores", requirePermission(dependencies.Authorizer, rbac.PermissionStoresRead), listStores(dependencies.Stores))
	api.GET("/assets", requirePermission(dependencies.Authorizer, rbac.PermissionAssetsRead), listAssets(dependencies.Assets))
	api.GET("/approvals", requirePermission(dependencies.Authorizer, rbac.PermissionApprovalsRead), listApprovals(dependencies.Approvals))
	api.GET("/integrations", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsRead), func(c *gin.Context) {
		statuses := dependencies.Integrations
		if statuses == nil {
			statuses = []integrations.Status{}
		}
		respondData(c, http.StatusOK, statuses)
	})
	if dependencies.Admin != nil {
		api.GET("/users", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), listUsers(dependencies.Admin))
		api.POST("/users", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), createUser(dependencies.Admin))
		api.PUT("/users/:id", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), updateUser(dependencies.Admin))
		api.PUT("/users/:id/roles", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), setUserRoles(dependencies.Admin, dependencies.PolicyReloader))
		api.GET("/roles", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), listRoles(dependencies.Admin))
		api.POST("/roles", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), createRole(dependencies.Admin))
		api.PUT("/roles/:id", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), updateRole(dependencies.Admin))
		api.DELETE("/roles/:id", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), deleteRole(dependencies.Admin, dependencies.PolicyReloader))
		api.PUT("/roles/:id/permissions", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), setRolePermissions(dependencies.Admin, dependencies.PolicyReloader))
		api.PUT("/roles/:id/menus", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), setRoleMenus(dependencies.Admin))
		api.GET("/permissions", requirePermission(dependencies.Authorizer, rbac.PermissionRBACManage), listPermissions(dependencies.Admin))
		api.GET("/menus/my", listMenus(dependencies.Admin, true))
		api.GET("/menus", requirePermission(dependencies.Authorizer, rbac.PermissionMenusManage), listMenus(dependencies.Admin, false))
		api.POST("/menus", requirePermission(dependencies.Authorizer, rbac.PermissionMenusManage), createMenu(dependencies.Admin))
		api.PUT("/menus/:id", requirePermission(dependencies.Authorizer, rbac.PermissionMenusManage), updateMenu(dependencies.Admin))
		api.DELETE("/menus/:id", requirePermission(dependencies.Authorizer, rbac.PermissionMenusManage), deleteMenu(dependencies.Admin))
		api.GET("/settings", requirePermission(dependencies.Authorizer, rbac.PermissionSettingsManage), listSettings(dependencies.Admin))
		api.PUT("/settings", requirePermission(dependencies.Authorizer, rbac.PermissionSettingsManage), upsertSetting(dependencies.Admin))
		api.DELETE("/settings/:namespace/:key", requirePermission(dependencies.Authorizer, rbac.PermissionSettingsManage), deleteSetting(dependencies.Admin))
	}
	if dependencies.IntegrationFlow != nil {
		dingTalkGet, dingTalkPut := integrationConfigHandlers(*dependencies.IntegrationFlow, "dingtalk")
		shopifyGet, shopifyPut := integrationConfigHandlers(*dependencies.IntegrationFlow, "shopify")
		api.GET("/integrations/dingtalk/config", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsRead), dingTalkGet)
		api.PUT("/integrations/dingtalk/config", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), dingTalkPut)
		api.GET("/integrations/dingtalk/users", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsRead), listDingTalkUsers(*dependencies.IntegrationFlow))
		api.GET("/integrations/dingtalk/login", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), dingTalkLogin(*dependencies.IntegrationFlow, true))
		api.GET("/integrations/shopify/config", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsRead), shopifyGet)
		api.PUT("/integrations/shopify/config", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), shopifyPut)
		api.GET("/integrations/shopify/install", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), shopifyInstall(*dependencies.IntegrationFlow))
		api.GET("/stores/:id", requirePermission(dependencies.Authorizer, rbac.PermissionStoresRead), getStore(dependencies.Admin))
		api.PUT("/stores/:id", requirePermission(dependencies.Authorizer, rbac.PermissionStoresWrite), updateStore(dependencies.Admin))
		api.POST("/stores/:id/disconnect", requirePermission(dependencies.Authorizer, rbac.PermissionStoresWrite), disconnectStore(dependencies.Admin))
		api.POST("/stores/:id/sync", requirePermission(dependencies.Authorizer, rbac.PermissionStoresWrite), syncStore(*dependencies.IntegrationFlow))
	} else {
		api.GET("/integrations/dingtalk/login", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), integrationBoundary(dependencies.Integrations, integrations.ProviderDingTalk))
		api.GET("/integrations/shopify/install", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), integrationBoundary(dependencies.Integrations, integrations.ProviderShopify))
	}

	return router
}

func integrationBoundary(statuses []integrations.Status, provider integrations.Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, status := range statuses {
			if status.Provider != provider {
				continue
			}
			if !status.Configured {
				respondError(c, http.StatusServiceUnavailable, "integration_not_configured", "The integration credentials are not configured.")
				return
			}
			respondError(c, http.StatusNotImplemented, "integration_not_implemented", "The integration adapter is scaffolded but its live provider flow is not implemented yet.")
			return
		}
		respondError(c, http.StatusServiceUnavailable, "integration_not_configured", "The integration is not registered.")
	}
}

func listStores(repository StoreRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		if repository == nil {
			respondData(c, http.StatusOK, []Store{})
			return
		}
		stores, err := repository.List(c.Request.Context(), principal.OrganizationID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", "The store list could not be loaded.")
			return
		}
		respondData(c, http.StatusOK, stores)
	}
}

func listAssets(repository AssetRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		if repository == nil {
			respondData(c, http.StatusOK, []Asset{})
			return
		}
		assets, err := repository.List(c.Request.Context(), principal.OrganizationID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", "The asset list could not be loaded.")
			return
		}
		respondData(c, http.StatusOK, assets)
	}
}

func listApprovals(repository ApprovalRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		if repository == nil {
			respondData(c, http.StatusOK, []Approval{})
			return
		}
		approvals, err := repository.List(c.Request.Context(), principal.OrganizationID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", "The approval list could not be loaded.")
			return
		}
		respondData(c, http.StatusOK, approvals)
	}
}
