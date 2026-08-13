package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xg-management/platform/backend/internal/rbac"
)

func (c *Client) LoadRBACPolicy(ctx context.Context) (rbac.PolicySnapshot, error) {
	var payload []byte
	err := c.pool.QueryRow(ctx, `
		SELECT jsonb_build_object(
			'policies', COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'role_id', r.id::text,
					'organization_id', r.organization_id::text,
					'permission', rp.permission_code
				) ORDER BY r.organization_id, r.id, rp.permission_code)
				FROM roles r
				JOIN role_permissions rp ON rp.role_id = r.id
			), '[]'::jsonb),
			'assignments', COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'user_id', ur.user_id::text,
					'role_id', ur.role_id::text,
					'organization_id', ur.organization_id::text
				) ORDER BY ur.organization_id, ur.user_id, ur.role_id)
				FROM user_roles ur
				JOIN roles r ON r.id = ur.role_id AND r.organization_id = ur.organization_id
			), '[]'::jsonb)
		)::text`).Scan(&payload)
	if err != nil {
		return rbac.PolicySnapshot{}, fmt.Errorf("query RBAC policy: %w", err)
	}
	return decodeRBACPolicySnapshot(payload)
}

func decodeRBACPolicySnapshot(payload []byte) (rbac.PolicySnapshot, error) {
	var wire struct {
		Policies []struct {
			RoleID         string `json:"role_id"`
			OrganizationID string `json:"organization_id"`
			Permission     string `json:"permission"`
		} `json:"policies"`
		Assignments []struct {
			UserID         string `json:"user_id"`
			RoleID         string `json:"role_id"`
			OrganizationID string `json:"organization_id"`
		} `json:"assignments"`
	}
	if err := json.Unmarshal(payload, &wire); err != nil {
		return rbac.PolicySnapshot{}, fmt.Errorf("decode RBAC policy: %w", err)
	}
	snapshot := rbac.PolicySnapshot{
		Policies:    make([]rbac.RolePolicy, 0, len(wire.Policies)),
		Assignments: make([]rbac.UserRole, 0, len(wire.Assignments)),
	}
	for _, item := range wire.Policies {
		snapshot.Policies = append(snapshot.Policies, rbac.RolePolicy{
			RoleID: item.RoleID, OrganizationID: item.OrganizationID, Permission: item.Permission,
		})
	}
	for _, item := range wire.Assignments {
		snapshot.Assignments = append(snapshot.Assignments, rbac.UserRole{
			UserID: item.UserID, RoleID: item.RoleID, OrganizationID: item.OrganizationID,
		})
	}
	return snapshot, nil
}
