package rbac

import (
	"context"
	"errors"
	"testing"

	"github.com/xg-management/platform/backend/internal/auth"
)

type policyStoreStub struct {
	snapshot PolicySnapshot
	err      error
}

func (s *policyStoreStub) LoadRBACPolicy(context.Context) (PolicySnapshot, error) {
	return s.snapshot, s.err
}

func principal(userID, organizationID string) auth.Principal {
	return auth.Principal{UserID: userID, OrganizationID: organizationID}
}

func TestAuthorizerAllowsPermissionThroughDomainRole(t *testing.T) {
	store := &policyStoreStub{snapshot: PolicySnapshot{
		Policies:    []RolePolicy{{RoleID: "operator", OrganizationID: "org-a", Permission: "stores:read"}},
		Assignments: []UserRole{{UserID: "alice", RoleID: "operator", OrganizationID: "org-a"}},
	}}
	authorizer, err := NewAuthorizer(context.Background(), store)
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}

	allowed, err := authorizer.Allowed(context.Background(), principal("alice", "org-a"), PermissionStoresRead)
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if !allowed {
		t.Fatal("Allowed() = false, want true")
	}
}

func TestAuthorizerRejectsMissingPermission(t *testing.T) {
	store := &policyStoreStub{snapshot: PolicySnapshot{
		Policies:    []RolePolicy{{RoleID: "viewer", OrganizationID: "org-a", Permission: "stores:read"}},
		Assignments: []UserRole{{UserID: "alice", RoleID: "viewer", OrganizationID: "org-a"}},
	}}
	authorizer, err := NewAuthorizer(context.Background(), store)
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}

	allowed, err := authorizer.Allowed(context.Background(), principal("alice", "org-a"), PermissionStoresWrite)
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if allowed {
		t.Fatal("Allowed() = true, want false")
	}
}

func TestAuthorizerIsolatesOrganizations(t *testing.T) {
	store := &policyStoreStub{snapshot: PolicySnapshot{
		Policies:    []RolePolicy{{RoleID: "owner", OrganizationID: "org-a", Permission: "*"}},
		Assignments: []UserRole{{UserID: "alice", RoleID: "owner", OrganizationID: "org-b"}},
	}}
	authorizer, err := NewAuthorizer(context.Background(), store)
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}

	allowed, err := authorizer.Allowed(context.Background(), principal("alice", "org-b"), PermissionRBACManage)
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if allowed {
		t.Fatal("Allowed() = true across organizations, want false")
	}
}

func TestAuthorizerAllowsOrganizationWildcardPermission(t *testing.T) {
	store := &policyStoreStub{snapshot: PolicySnapshot{
		Policies:    []RolePolicy{{RoleID: "owner", OrganizationID: "org-a", Permission: "*"}},
		Assignments: []UserRole{{UserID: "alice", RoleID: "owner", OrganizationID: "org-a"}},
	}}
	authorizer, err := NewAuthorizer(context.Background(), store)
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}

	allowed, err := authorizer.Allowed(context.Background(), principal("alice", "org-a"), PermissionSettingsManage)
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if !allowed {
		t.Fatal("Allowed() = false for organization wildcard, want true")
	}
}

func TestAuthorizerReloadsChangedPolicies(t *testing.T) {
	store := &policyStoreStub{snapshot: PolicySnapshot{
		Policies:    []RolePolicy{{RoleID: "viewer", OrganizationID: "org-a", Permission: "stores:read"}},
		Assignments: []UserRole{{UserID: "alice", RoleID: "viewer", OrganizationID: "org-a"}},
	}}
	authorizer, err := NewAuthorizer(context.Background(), store)
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	store.snapshot.Policies = append(store.snapshot.Policies, RolePolicy{RoleID: "viewer", OrganizationID: "org-a", Permission: "stores:write"})
	if err := authorizer.Reload(context.Background()); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	allowed, err := authorizer.Allowed(context.Background(), principal("alice", "org-a"), PermissionStoresWrite)
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if !allowed {
		t.Fatal("Allowed() = false after policy reload, want true")
	}
}

func TestAuthorizerFailsClosedAfterReloadError(t *testing.T) {
	store := &policyStoreStub{snapshot: PolicySnapshot{
		Policies:    []RolePolicy{{RoleID: "owner", OrganizationID: "org-a", Permission: "*"}},
		Assignments: []UserRole{{UserID: "alice", RoleID: "owner", OrganizationID: "org-a"}},
	}}
	authorizer, err := NewAuthorizer(context.Background(), store)
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}
	store.err = errors.New("database unavailable")
	if err := authorizer.Reload(context.Background()); err == nil {
		t.Fatal("Reload() error = nil, want load failure")
	}

	allowed, err := authorizer.Allowed(context.Background(), principal("alice", "org-a"), PermissionSettingsManage)
	if err == nil {
		t.Fatal("Allowed() error = nil after failed reload, want fail-closed error")
	}
	if allowed {
		t.Fatal("Allowed() = true after failed reload, want false")
	}
}
