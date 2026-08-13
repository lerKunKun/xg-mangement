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
		"('shopify:sync', 'Synchronize Shopify store resources')",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("migration missing %q", fragment)
		}
	}
}

func TestShopifyMirrorHardeningMigration(t *testing.T) {
	body, err := files.ReadFile("000004_shopify_mirror_hardening.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"CREATE UNIQUE INDEX shopify_stores_domain_global_unique",
		"ADD COLUMN lease_owner TEXT",
		"ADD COLUMN resync_requested BOOLEAN",
		"shopify_variants_product_tenant_fk",
		"'ARCHIVED'",
		"'LOCKED'",
	} {
		if !strings.Contains(string(body), fragment) {
			t.Fatalf("hardening migration missing %q", fragment)
		}
	}
}
