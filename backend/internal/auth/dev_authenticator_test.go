package auth_test

import (
	"net/http/httptest"
	"testing"

	"github.com/xg-management/platform/backend/internal/auth"
)

func TestDevAuthenticatorIsOptIn(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/me", nil)
	request.Header.Set("X-Dev-User-ID", "user-1")
	request.Header.Set("X-Dev-Organization-ID", "org-1")

	_, ok := (auth.DevAuthenticator{Enabled: false}).Authenticate(request)
	if ok {
		t.Fatal("Authenticate() ok = true while development authentication is disabled")
	}
}

func TestDevAuthenticatorBuildsPrincipalFromHeaders(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/me", nil)
	request.Header.Set("X-Dev-User-ID", "user-1")
	request.Header.Set("X-Dev-Organization-ID", "org-1")
	request.Header.Set("X-Dev-Display-Name", "Local Owner")
	request.Header.Set("X-Dev-Permissions", "stores:read, assets:read")

	principal, ok := (auth.DevAuthenticator{Enabled: true}).Authenticate(request)
	if !ok {
		t.Fatal("Authenticate() ok = false, want true")
	}
	if principal.OrganizationID != "org-1" || principal.UserID != "user-1" {
		t.Fatalf("principal = %#v, want user-1 in org-1", principal)
	}
	if len(principal.Permissions) != 2 || principal.Permissions[1] != "assets:read" {
		t.Fatalf("permissions = %#v, want normalized header permissions", principal.Permissions)
	}
}

func TestDevAuthenticatorRejectsIncompleteIdentity(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/me", nil)
	request.Header.Set("X-Dev-User-ID", "user-1")

	_, ok := (auth.DevAuthenticator{Enabled: true}).Authenticate(request)
	if ok {
		t.Fatal("Authenticate() ok = true with missing organization header")
	}
}
