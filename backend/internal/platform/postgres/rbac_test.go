package postgres

import (
	"testing"
)

func TestDecodeRBACPolicySnapshot(t *testing.T) {
	payload := []byte(`{
		"policies": [
			{"role_id":"role-owner","organization_id":"org-a","permission":"*"},
			{"role_id":"role-viewer","organization_id":"org-b","permission":"stores:read"}
		],
		"assignments": [
			{"user_id":"alice","role_id":"role-owner","organization_id":"org-a"},
			{"user_id":"alice","role_id":"role-viewer","organization_id":"org-b"}
		]
	}`)

	snapshot, err := decodeRBACPolicySnapshot(payload)
	if err != nil {
		t.Fatalf("decodeRBACPolicySnapshot() error = %v", err)
	}
	if len(snapshot.Policies) != 2 {
		t.Fatalf("policies length = %d, want 2", len(snapshot.Policies))
	}
	if got := snapshot.Policies[0]; got.RoleID != "role-owner" || got.OrganizationID != "org-a" || got.Permission != "*" {
		t.Fatalf("first policy = %#v", got)
	}
	if len(snapshot.Assignments) != 2 {
		t.Fatalf("assignments length = %d, want 2", len(snapshot.Assignments))
	}
	if got := snapshot.Assignments[1]; got.UserID != "alice" || got.RoleID != "role-viewer" || got.OrganizationID != "org-b" {
		t.Fatalf("second assignment = %#v", got)
	}
}

func TestDecodeRBACPolicySnapshotRejectsInvalidPayload(t *testing.T) {
	if _, err := decodeRBACPolicySnapshot([]byte(`{"policies":`)); err == nil {
		t.Fatal("decodeRBACPolicySnapshot() error = nil, want invalid JSON error")
	}
}
