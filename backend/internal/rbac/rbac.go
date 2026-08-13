package rbac

import "github.com/xg-management/platform/backend/internal/auth"

type Permission string

const (
	PermissionStoresRead         Permission = "stores:read"
	PermissionStoresWrite        Permission = "stores:write"
	PermissionAssetsRead         Permission = "assets:read"
	PermissionAssetsWrite        Permission = "assets:write"
	PermissionApprovalsRead      Permission = "approvals:read"
	PermissionApprovalsRequest   Permission = "approvals:request"
	PermissionIntegrationsRead   Permission = "integrations:read"
	PermissionIntegrationsManage Permission = "integrations:manage"
	PermissionReportsRead        Permission = "reports:read"
	PermissionRBACManage         Permission = "rbac:manage"
)

type Authorizer struct{}

func (Authorizer) Allowed(principal auth.Principal, required Permission) bool {
	for _, granted := range principal.Permissions {
		if granted == "*" || granted == string(required) {
			return true
		}
	}
	return false
}
