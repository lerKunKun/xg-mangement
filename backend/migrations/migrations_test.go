package migrations

import (
	"strings"
	"testing"
)

func TestShopifyMirrorMigrationContainsTenantScopedTables(t *testing.T) {
	sql, err := files.ReadFile("000003_shopify_mirror_runtime.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	body := string(sql)
	for _, fragment := range []string{
		"CREATE TABLE shopify_sync_runs",
		"UNIQUE (organization_id, store_id, shopify_gid)",
		"CREATE TABLE shopify_themes",
		"CREATE TABLE outbox_messages",
		"CHECK (status IN ('queued', 'running', 'completed', 'failed'))",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}
