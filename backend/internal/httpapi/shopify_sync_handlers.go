package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/shopifysync"
)

type ShopifySyncRepository interface {
	CreateSyncRun(context.Context, string, string, string, shopifysync.SyncMode) (shopifysync.SyncRun, error)
	ListSyncRuns(context.Context, string, string, int) ([]shopifysync.SyncRun, error)
	ListThemes(context.Context, string, string) ([]shopifysync.Theme, error)
}

func createShopifySyncRun(repository ShopifySyncRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Mode shopifysync.SyncMode `json:"mode"`
		}
		if c.Request.ContentLength > 0 && !bindJSON(c, &input) {
			return
		}
		if input.Mode == "" {
			input.Mode = shopifysync.SyncModeFull
		}
		if input.Mode != shopifysync.SyncModeFull {
			respondError(c, http.StatusBadRequest, "invalid_sync_mode", "Only a full manual synchronization can be requested.")
			return
		}
		principal, _ := currentPrincipal(c)
		run, err := repository.CreateSyncRun(c.Request.Context(), principal.OrganizationID, strings.TrimSpace(c.Param("id")), principal.UserID, input.Mode)
		if errors.Is(err, shopifysync.ErrSyncAlreadyRunning) {
			respondError(c, http.StatusConflict, "sync_already_running", "This store already has an active synchronization.")
			return
		}
		if err != nil {
			respondError(c, http.StatusInternalServerError, "sync_create_failed", "The Shopify synchronization could not be queued.")
			return
		}
		respondData(c, http.StatusAccepted, run)
	}
}

func listShopifySyncRuns(repository ShopifySyncRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		runs, err := repository.ListSyncRuns(c.Request.Context(), principal.OrganizationID, c.Param("id"), 50)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "sync_runs_unavailable", "The Shopify synchronization history could not be loaded.")
			return
		}
		respondData(c, http.StatusOK, runs)
	}
}

func listShopifyThemes(repository ShopifySyncRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		themes, err := repository.ListThemes(c.Request.Context(), principal.OrganizationID, c.Param("id"))
		if err != nil {
			respondError(c, http.StatusInternalServerError, "themes_unavailable", "The Shopify themes could not be loaded.")
			return
		}
		respondData(c, http.StatusOK, themes)
	}
}
