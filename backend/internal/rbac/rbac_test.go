package rbac_test

import (
	"context"
	"testing"

	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/rbac"
)

type policyStore struct{ snapshot rbac.PolicySnapshot }

func (s policyStore) LoadRBACPolicy(context.Context) (rbac.PolicySnapshot, error) {
	return s.snapshot, nil
}

func TestAuthorizerPublicContractUsesPersistedRoles(t *testing.T) {
	authorizer, err := rbac.NewAuthorizer(context.Background(), policyStore{snapshot: rbac.PolicySnapshot{
		Policies:    []rbac.RolePolicy{{RoleID: "operator", OrganizationID: "org-a", Permission: string(rbac.PermissionStoresWrite)}},
		Assignments: []rbac.UserRole{{UserID: "alice", RoleID: "operator", OrganizationID: "org-a"}},
	}})
	if err != nil {
		t.Fatalf("NewAuthorizer() error = %v", err)
	}

	allowed, err := authorizer.Allowed(context.Background(), auth.Principal{
		UserID: "alice", OrganizationID: "org-a", Permissions: []string{},
	}, rbac.PermissionStoresWrite)
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if !allowed {
		t.Fatal("Allowed() = false, want persisted Casbin policy to grant access")
	}
}
