package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/httpapi"
	"github.com/xg-management/platform/backend/internal/integrations"
	"github.com/xg-management/platform/backend/internal/rbac"
)

func TestProtectedRouteRejectsUnauthenticatedRequest(t *testing.T) {
	router := newTestRouter(auth.Principal{}, false, &storeRepositoryStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/stores", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	assertErrorCode(t, response, "unauthenticated")
}

func TestProtectedRouteRejectsMissingPermission(t *testing.T) {
	principal := auth.Principal{
		UserID:         "user-1",
		OrganizationID: "org-1",
		Permissions:    []string{string(rbac.PermissionAssetsRead)},
	}
	router := newTestRouter(principal, true, &storeRepositoryStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/stores", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
	assertErrorCode(t, response, "permission_denied")
}

func TestStoresRouteUsesAuthenticatedOrganization(t *testing.T) {
	principal := auth.Principal{
		UserID:         "user-1",
		OrganizationID: "org-trusted",
		DisplayName:    "Development Owner",
		Permissions:    []string{string(rbac.PermissionStoresRead)},
	}
	repository := &storeRepositoryStub{
		stores: []httpapi.Store{{ID: "store-1", Name: "Primary Store", Domain: "primary.myshopify.com", Status: "connected"}},
	}
	router := newTestRouter(principal, true, repository)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/stores?organization_id=org-attacker", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if repository.organizationID != "org-trusted" {
		t.Fatalf("repository organization = %q, want org-trusted", repository.organizationID)
	}

	var envelope struct {
		Data []httpapi.Store `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].Domain != "primary.myshopify.com" {
		t.Fatalf("data = %#v, want repository stores", envelope.Data)
	}
}

func TestRequestIDIsPreserved(t *testing.T) {
	router := newTestRouter(auth.Principal{}, false, &storeRepositoryStub{})
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Request-ID", "request-123")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if got := response.Header().Get("X-Request-ID"); got != "request-123" {
		t.Fatalf("X-Request-ID = %q, want request-123", got)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestIntegrationEntryPointReportsConfigurationState(t *testing.T) {
	principal := auth.Principal{
		UserID:         "user-1",
		OrganizationID: "org-1",
		Permissions:    []string{string(rbac.PermissionIntegrationsManage)},
	}
	tests := []struct {
		name       string
		configured bool
		wantStatus int
		wantCode   string
	}{
		{name: "missing credentials", configured: false, wantStatus: http.StatusServiceUnavailable, wantCode: "integration_not_configured"},
		{name: "adapter pending implementation", configured: true, wantStatus: http.StatusNotImplemented, wantCode: "integration_not_implemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := httpapi.NewRouter(httpapi.Dependencies{
				Authenticator: authenticatorStub{principal: principal, authenticated: true},
				Authorizer:    rbac.Authorizer{},
				Integrations: []integrations.Status{{
					Provider:   integrations.ProviderShopify,
					Configured: tt.configured,
				}},
			})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/integrations/shopify/install", nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tt.wantStatus, response.Body.String())
			}
			assertErrorCode(t, response, tt.wantCode)
		})
	}
}

func newTestRouter(principal auth.Principal, authenticated bool, stores httpapi.StoreRepository) http.Handler {
	return httpapi.NewRouter(httpapi.Dependencies{
		Authenticator: authenticatorStub{principal: principal, authenticated: authenticated},
		Authorizer:    rbac.Authorizer{},
		Stores:        stores,
		Assets:        assetRepositoryStub{},
		Approvals:     approvalRepositoryStub{},
	})
}

type authenticatorStub struct {
	principal     auth.Principal
	authenticated bool
}

func (a authenticatorStub) Authenticate(*http.Request) (auth.Principal, bool) {
	return a.principal, a.authenticated
}

type storeRepositoryStub struct {
	stores         []httpapi.Store
	organizationID string
}

func (s *storeRepositoryStub) List(_ context.Context, organizationID string) ([]httpapi.Store, error) {
	s.organizationID = organizationID
	return s.stores, nil
}

type assetRepositoryStub struct{}

func (assetRepositoryStub) List(context.Context, string) ([]httpapi.Asset, error) {
	return []httpapi.Asset{}, nil
}

type approvalRepositoryStub struct{}

func (approvalRepositoryStub) List(context.Context, string) ([]httpapi.Approval, error) {
	return []httpapi.Approval{}, nil
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if envelope.Error.Code != want {
		t.Fatalf("error code = %q, want %q", envelope.Error.Code, want)
	}
}
