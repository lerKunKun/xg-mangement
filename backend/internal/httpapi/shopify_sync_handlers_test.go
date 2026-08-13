package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/httpapi"
	"github.com/xg-management/platform/backend/internal/rbac"
	"github.com/xg-management/platform/backend/internal/shopifysync"
)

func TestCreateSyncRunUsesPrincipalOrganization(t *testing.T) {
	principal := auth.Principal{UserID: "user-1", OrganizationID: "org-trusted", Permissions: []string{string(rbac.PermissionShopifySync)}}
	repository := &syncRunRepositoryStub{}
	router := httpapi.NewRouter(httpapi.Dependencies{Authenticator: authenticatorStub{principal: principal, authenticated: true}, Authorizer: authorizerStub{}, ShopifySync: repository})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/stores/store-1/sync-runs?organization_id=org-attacker", strings.NewReader(`{"mode":"full"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repository.organizationID != "org-trusted" || repository.userID != "user-1" || repository.storeID != "store-1" {
		t.Fatalf("repository args = org %q user %q store %q", repository.organizationID, repository.userID, repository.storeID)
	}
}

func TestCreateSyncRunRequiresShopifySyncPermission(t *testing.T) {
	principal := auth.Principal{UserID: "viewer", OrganizationID: "org-1", Permissions: []string{string(rbac.PermissionStoresRead)}}
	router := httpapi.NewRouter(httpapi.Dependencies{Authenticator: authenticatorStub{principal: principal, authenticated: true}, Authorizer: authorizerStub{}, ShopifySync: &syncRunRepositoryStub{}})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/stores/store-1/sync-runs", strings.NewReader(`{"mode":"full"}`)))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestListSyncRunsAndThemesRequireStoresRead(t *testing.T) {
	principal := auth.Principal{UserID: "viewer", OrganizationID: "org-1", Permissions: []string{string(rbac.PermissionStoresRead)}}
	repository := &syncRunRepositoryStub{}
	router := httpapi.NewRouter(httpapi.Dependencies{Authenticator: authenticatorStub{principal: principal, authenticated: true}, Authorizer: authorizerStub{}, ShopifySync: repository})
	for _, path := range []string{"/api/v1/stores/store-1/sync-runs", "/api/v1/stores/store-1/themes"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", path, response.Code, response.Body.String())
		}
	}
}

func TestCreateSyncRunReportsActiveConflict(t *testing.T) {
	principal := auth.Principal{UserID: "user-1", OrganizationID: "org-1", Permissions: []string{string(rbac.PermissionShopifySync)}}
	repository := &syncRunRepositoryStub{createErr: shopifysync.ErrSyncAlreadyRunning}
	router := httpapi.NewRouter(httpapi.Dependencies{Authenticator: authenticatorStub{principal: principal, authenticated: true}, Authorizer: authorizerStub{}, ShopifySync: repository})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/stores/store-1/sync-runs", strings.NewReader(`{"mode":"full"}`)))
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assertErrorCode(t, response, "sync_already_running")
}

func TestCreateSyncRunReportsMissingOrDisconnectedStore(t *testing.T) {
	principal := auth.Principal{UserID: "user-1", OrganizationID: "org-1", Permissions: []string{string(rbac.PermissionShopifySync)}}
	repository := &syncRunRepositoryStub{createErr: shopifysync.ErrStoreNotConnected}
	router := httpapi.NewRouter(httpapi.Dependencies{Authenticator: authenticatorStub{principal: principal, authenticated: true}, Authorizer: authorizerStub{}, ShopifySync: repository})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/stores/store-missing/sync-runs", strings.NewReader(`{"mode":"full"}`)))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	assertErrorCode(t, response, "store_not_connected")
}

type syncRunRepositoryStub struct {
	organizationID string
	storeID        string
	userID         string
	createErr      error
}

func (r *syncRunRepositoryStub) CreateSyncRun(_ context.Context, organizationID, storeID, requestedBy string, mode shopifysync.SyncMode) (shopifysync.SyncRun, error) {
	r.organizationID, r.storeID, r.userID = organizationID, storeID, requestedBy
	return shopifysync.SyncRun{ID: "run-1", StoreID: storeID, Mode: mode, Status: shopifysync.SyncStatusQueued, CreatedAt: time.Now()}, r.createErr
}
func (r *syncRunRepositoryStub) ListSyncRuns(context.Context, string, string, int) ([]shopifysync.SyncRun, error) {
	return []shopifysync.SyncRun{}, nil
}
func (r *syncRunRepositoryStub) ListThemes(context.Context, string, string) ([]shopifysync.Theme, error) {
	return []shopifysync.Theme{}, nil
}

var _ = json.Valid
