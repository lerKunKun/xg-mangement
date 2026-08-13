package rbac

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/xg-management/platform/backend/internal/auth"
)

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
	PermissionSettingsManage     Permission = "settings:manage"
	PermissionMenusManage        Permission = "menus:manage"
	PermissionShopifySync        Permission = "shopify:sync"
)

type RolePolicy struct {
	RoleID         string
	OrganizationID string
	Permission     string
}

type UserRole struct {
	UserID         string
	RoleID         string
	OrganizationID string
}

type PolicySnapshot struct {
	Policies    []RolePolicy
	Assignments []UserRole
}

type PolicyStore interface {
	LoadRBACPolicy(context.Context) (PolicySnapshot, error)
}

const modelText = `[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && (p.obj == "*" || r.obj == p.obj) && r.act == p.act
`

type Authorizer struct {
	store  PolicyStore
	mu     sync.RWMutex
	engine *casbin.Enforcer
}

func NewAuthorizer(ctx context.Context, store PolicyStore) (*Authorizer, error) {
	if store == nil {
		return nil, fmt.Errorf("RBAC policy store is required")
	}
	authorizer := &Authorizer{store: store}
	if err := authorizer.Reload(ctx); err != nil {
		return nil, err
	}
	return authorizer, nil
}

func (a *Authorizer) Reload(ctx context.Context) error {
	if a == nil || a.store == nil {
		return fmt.Errorf("RBAC authorizer is not configured")
	}
	a.mu.Lock()
	a.engine = nil
	a.mu.Unlock()
	snapshot, err := a.store.LoadRBACPolicy(ctx)
	if err != nil {
		return fmt.Errorf("load RBAC policy: %w", err)
	}
	engine, err := newEngine(snapshot)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.engine = engine
	a.mu.Unlock()
	return nil
}

func (a *Authorizer) Allowed(_ context.Context, principal auth.Principal, required Permission) (bool, error) {
	if a == nil || strings.TrimSpace(principal.UserID) == "" || strings.TrimSpace(principal.OrganizationID) == "" || strings.TrimSpace(string(required)) == "" {
		return false, nil
	}
	a.mu.RLock()
	engine := a.engine
	if engine == nil {
		a.mu.RUnlock()
		return false, fmt.Errorf("RBAC policy is not loaded")
	}
	allowed, err := engine.Enforce(userSubject(principal.UserID), principal.OrganizationID, string(required), "use")
	a.mu.RUnlock()
	if err != nil {
		return false, fmt.Errorf("enforce RBAC policy: %w", err)
	}
	return allowed, nil
}

func newEngine(snapshot PolicySnapshot) (*casbin.Enforcer, error) {
	policyModel, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("create Casbin model: %w", err)
	}
	engine, err := casbin.NewEnforcer(policyModel)
	if err != nil {
		return nil, fmt.Errorf("create Casbin enforcer: %w", err)
	}
	for _, policy := range snapshot.Policies {
		if policy.RoleID == "" || policy.OrganizationID == "" || policy.Permission == "" {
			return nil, fmt.Errorf("invalid role policy")
		}
		if _, err := engine.AddPolicy(roleSubject(policy.RoleID), policy.OrganizationID, policy.Permission, "use"); err != nil {
			return nil, fmt.Errorf("add role policy: %w", err)
		}
	}
	for _, assignment := range snapshot.Assignments {
		if assignment.UserID == "" || assignment.RoleID == "" || assignment.OrganizationID == "" {
			return nil, fmt.Errorf("invalid user role assignment")
		}
		if _, err := engine.AddGroupingPolicy(userSubject(assignment.UserID), roleSubject(assignment.RoleID), assignment.OrganizationID); err != nil {
			return nil, fmt.Errorf("add user role assignment: %w", err)
		}
	}
	return engine, nil
}

func userSubject(id string) string { return "user::" + id }
func roleSubject(id string) string { return "role::" + id }
