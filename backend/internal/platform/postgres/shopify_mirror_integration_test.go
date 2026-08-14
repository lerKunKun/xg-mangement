package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/shopifysync"
)

const localTestOrganizationID = "00000000-0000-0000-0000-000000000001"

func TestShopifySyncLeaseRecoveryAndAtomicCompletion(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_TEST_URL is not set")
	}
	ctx := context.Background()
	client, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	storeID, runID := createIntegrationSyncRun(t, client, "lease")
	defer cleanupIntegrationSyncStore(ctx, client, storeID)
	request := shopifysync.SyncRequest{OrganizationID: localTestOrganizationID, StoreID: storeID, RunID: runID, Mode: shopifysync.SyncModeFull}
	firstLease, err := client.StartSyncRun(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.StartSyncRun(ctx, request); !errors.Is(err, shopifysync.ErrSyncAlreadyRunning) {
		t.Fatalf("concurrent StartSyncRun() error = %v", err)
	}
	if _, err := client.pool.Exec(ctx, `UPDATE shopify_sync_runs SET lease_expires_at=now()-interval '1 second' WHERE id=$1`, runID); err != nil {
		t.Fatal(err)
	}
	secondLease, err := client.StartSyncRun(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if firstLease == secondLease {
		t.Fatal("expired lease was not replaced")
	}
	request.LeaseOwner = firstLease
	if err := client.HeartbeatSyncRun(ctx, request); err == nil {
		t.Fatal("stale lease owner renewed the sync run")
	}
	request.LeaseOwner = secondLease
	if err := client.ReplaceMirror(ctx, request, shopifysync.MirrorBatch{SyncedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	var status string
	var leaseOwner *string
	if err := client.pool.QueryRow(ctx, `SELECT status,lease_owner FROM shopify_sync_runs WHERE id=$1`, runID).Scan(&status, &leaseOwner); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || leaseOwner != nil {
		t.Fatalf("completed run state = status %q lease %#v", status, leaseOwner)
	}
}

func TestShopifySyncCompletionEnqueuesRequestedFollowUp(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_TEST_URL is not set")
	}
	ctx := context.Background()
	client, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	storeID, runID := createIntegrationSyncRun(t, client, "resync")
	defer cleanupIntegrationSyncStore(ctx, client, storeID)
	request := shopifysync.SyncRequest{OrganizationID: localTestOrganizationID, StoreID: storeID, RunID: runID, Mode: shopifysync.SyncModeFull}
	request.LeaseOwner, err = client.StartSyncRun(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.pool.Exec(ctx, `UPDATE shopify_stores SET resync_requested=true WHERE id=$1`, storeID); err != nil {
		t.Fatal(err)
	}
	if err := client.ReplaceMirror(ctx, request, shopifysync.MirrorBatch{SyncedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	var queued, outbox int
	if err := client.pool.QueryRow(ctx, `SELECT count(*) FROM shopify_sync_runs WHERE store_id=$1 AND status='queued'`, storeID).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if err := client.pool.QueryRow(ctx, `SELECT count(*) FROM outbox_messages WHERE envelope->'payload'->>'store_id'=$1 AND status='pending'`, storeID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if queued != 1 || outbox != 1 {
		t.Fatalf("follow-up state = queued %d outbox %d", queued, outbox)
	}
}

func TestShopifyStoreDomainIsGloballyCaseInsensitiveUnique(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_TEST_URL is not set")
	}
	ctx := context.Background()
	client, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	domain := "codex-domain-" + time.Now().UTC().Format("20060102150405.000000000") + ".myshopify.com"
	var storeID string
	if err := client.pool.QueryRow(ctx, `INSERT INTO shopify_stores(organization_id,name,shop_domain,status) VALUES($1,'Domain Test',$2,'connected') RETURNING id::text`, localTestOrganizationID, domain).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	defer cleanupIntegrationSyncStore(ctx, client, storeID)
	if _, err := client.pool.Exec(ctx, `INSERT INTO shopify_stores(organization_id,name,shop_domain,status) VALUES($1,'Duplicate Domain',$2,'connected')`, localTestOrganizationID, strings.ToUpper(domain)); err == nil {
		t.Fatal("case-insensitive duplicate Shopify domain was accepted")
	}
}

func TestPendingShopifyStoreIsListedBeforeOAuthCompletes(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_TEST_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_TEST_URL is not set")
	}
	ctx := context.Background()
	client, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	domain := "codex-pending-" + time.Now().UTC().Format("20060102150405.000000000") + ".myshopify.com"

	store, err := client.EnsurePendingShopifyStore(ctx, localTestOrganizationID, domain)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupIntegrationSyncStore(ctx, client, store.ID)
	if store.Domain != domain || store.Status != "pending" {
		t.Fatalf("pending store = %#v", store)
	}

	stores, err := client.List(ctx, localTestOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range stores {
		if item.ID == store.ID && item.Domain == domain && item.Status == "pending" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("pending store %s was not returned by List", store.ID)
	}
}

func createIntegrationSyncRun(t *testing.T, client *Client, suffix string) (string, string) {
	t.Helper()
	ctx := context.Background()
	var storeID, runID string
	domain := "codex-" + suffix + "-" + time.Now().UTC().Format("20060102150405.000000000") + ".myshopify.com"
	if err := client.pool.QueryRow(ctx, `
		INSERT INTO shopify_stores(organization_id,name,shop_domain,status)
		VALUES($1,'Integration Test',$2,'connected') RETURNING id::text`, localTestOrganizationID, domain).Scan(&storeID); err != nil {
		t.Fatal(err)
	}
	if err := client.pool.QueryRow(ctx, `
		INSERT INTO shopify_sync_runs(organization_id,store_id,mode,status,job_id)
		VALUES($1,$2,'full','queued',gen_random_uuid()::text) RETURNING id::text`, localTestOrganizationID, storeID).Scan(&runID); err != nil {
		_, _ = client.pool.Exec(ctx, `DELETE FROM shopify_stores WHERE id=$1`, storeID)
		t.Fatal(err)
	}
	return storeID, runID
}

func cleanupIntegrationSyncStore(ctx context.Context, client *Client, storeID string) {
	_, _ = client.pool.Exec(ctx, `DELETE FROM outbox_messages WHERE envelope->'payload'->>'store_id'=$1`, storeID)
	_, _ = client.pool.Exec(ctx, `DELETE FROM shopify_stores WHERE id=$1`, storeID)
}
