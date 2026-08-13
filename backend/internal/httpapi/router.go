package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/integrations"
	"github.com/xg-management/platform/backend/internal/rbac"
)

type Authenticator interface {
	Authenticate(*http.Request) (auth.Principal, bool)
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
	Authenticator Authenticator
	Authorizer    rbac.Authorizer
	Stores        StoreRepository
	Assets        AssetRepository
	Approvals     ApprovalRepository
	Integrations  []integrations.Status
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
	router.GET("/api/v1/integrations/dingtalk/callback", integrationBoundary(dependencies.Integrations, integrations.ProviderDingTalk))
	router.GET("/api/v1/integrations/shopify/callback", integrationBoundary(dependencies.Integrations, integrations.ProviderShopify))
	router.POST("/api/v1/webhooks/shopify", integrationBoundary(dependencies.Integrations, integrations.ProviderShopify))

	api := router.Group("/api/v1")
	api.Use(authenticate(dependencies.Authenticator))
	api.GET("/me", func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		respondData(c, http.StatusOK, principal)
	})
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
	api.GET("/integrations/dingtalk/login", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), integrationBoundary(dependencies.Integrations, integrations.ProviderDingTalk))
	api.GET("/integrations/shopify/install", requirePermission(dependencies.Authorizer, rbac.PermissionIntegrationsManage), integrationBoundary(dependencies.Integrations, integrations.ProviderShopify))

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
