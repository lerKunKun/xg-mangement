package rbac_test

import (
	"testing"

	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/rbac"
)

func TestAuthorizerAllowed(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    rbac.Permission
		want        bool
	}{
		{
			name:        "owner wildcard can manage access",
			permissions: []string{"*"},
			required:    rbac.PermissionRBACManage,
			want:        true,
		},
		{
			name:        "operator can write stores",
			permissions: []string{string(rbac.PermissionStoresRead), string(rbac.PermissionStoresWrite)},
			required:    rbac.PermissionStoresWrite,
			want:        true,
		},
		{
			name:        "viewer cannot write stores",
			permissions: []string{string(rbac.PermissionStoresRead)},
			required:    rbac.PermissionStoresWrite,
			want:        false,
		},
	}

	authorizer := rbac.Authorizer{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal := auth.Principal{Permissions: tt.permissions}
			if got := authorizer.Allowed(principal, tt.required); got != tt.want {
				t.Fatalf("Allowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
