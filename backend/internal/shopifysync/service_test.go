package shopifysync

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/integrations/shopify"
)

func TestServiceSyncReplacesMirrorOnlyAfterAllSourcesComplete(t *testing.T) {
	sequence := []string{}
	repository := &syncRepositoryStub{sequence: &sequence}
	bulk := &bulkClientStub{sequence: &sequence}
	service := Service{
		Repository: repository,
		Tokens:     accessTokenStub{sequence: &sequence},
		Stores:     storeResolverStub{},
		Shopify:    bulk,
		Sleeper:    func(context.Context, time.Duration) error { return nil },
	}
	request := SyncRequest{OrganizationID: "org-1", StoreID: "store-1", RunID: "run-1", Mode: SyncModeFull}

	if err := service.Sync(context.Background(), request); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	want := []string{"start-run", "access-token", "start-products", "poll-products", "download-products", "start-collections", "poll-collections", "download-collections", "list-themes", "heartbeat", "replace-mirror"}
	if strings.Join(sequence, ",") != strings.Join(want, ",") {
		t.Fatalf("sequence = %v, want %v", sequence, want)
	}
	if repository.replaced.Counts() != (ResourceCounts{Products: 1, Variants: 1, Collections: 1, Themes: 1}) {
		t.Fatalf("counts = %#v", repository.replaced.Counts())
	}
}

func TestServiceSyncDoesNotReplaceMirrorWhenBulkFails(t *testing.T) {
	sequence := []string{}
	repository := &syncRepositoryStub{sequence: &sequence}
	bulk := &bulkClientStub{sequence: &sequence, failProducts: true}
	service := Service{Repository: repository, Tokens: accessTokenStub{sequence: &sequence}, Stores: storeResolverStub{}, Shopify: bulk, Sleeper: func(context.Context, time.Duration) error { return nil }}

	err := service.Sync(context.Background(), SyncRequest{OrganizationID: "org-1", StoreID: "store-1", RunID: "run-1", Mode: SyncModeFull})
	if err == nil {
		t.Fatal("Sync() error = nil")
	}
	if repository.replaceCalls != 0 || repository.failCalls != 1 {
		t.Fatalf("replace calls = %d, fail calls = %d", repository.replaceCalls, repository.failCalls)
	}
}

func TestServiceSyncTreatsCompletedRunAsIdempotentSuccess(t *testing.T) {
	sequence := []string{}
	repository := &syncRepositoryStub{sequence: &sequence, startErr: ErrSyncAlreadyCompleted}
	service := Service{Repository: repository, Tokens: accessTokenStub{sequence: &sequence}, Stores: storeResolverStub{}, Shopify: &bulkClientStub{sequence: &sequence}}

	if err := service.Sync(context.Background(), SyncRequest{OrganizationID: "org-1", StoreID: "store-1", RunID: "run-1", Mode: SyncModeFull}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if strings.Join(sequence, ",") != "start-run" || repository.failCalls != 0 {
		t.Fatalf("sequence = %v, fail calls = %d", sequence, repository.failCalls)
	}
}

func TestServiceSyncClassifiesConcurrentRunAsRetryable(t *testing.T) {
	sequence := []string{}
	repository := &syncRepositoryStub{sequence: &sequence, startErr: ErrSyncAlreadyRunning}
	service := Service{Repository: repository, Tokens: accessTokenStub{sequence: &sequence}, Stores: storeResolverStub{}, Shopify: &bulkClientStub{sequence: &sequence}}

	err := service.Sync(context.Background(), SyncRequest{OrganizationID: "org-1", StoreID: "store-1", RunID: "run-1", Mode: SyncModeFull})
	var syncErr *Error
	if !errors.As(err, &syncErr) || !syncErr.Retryable || syncErr.Code != "sync_already_running" {
		t.Fatalf("Sync() error = %#v", err)
	}
}

func TestServiceSyncConvertsPanicAndFailsOwnedRun(t *testing.T) {
	sequence := []string{}
	repository := &syncRepositoryStub{sequence: &sequence}
	service := Service{
		Repository: repository,
		Tokens:     accessTokenPanicStub{},
		Stores:     storeResolverStub{},
		Shopify:    &bulkClientStub{sequence: &sequence},
	}
	err := service.Sync(context.Background(), SyncRequest{OrganizationID: "org-1", StoreID: "store-1", RunID: "run-1", Mode: SyncModeFull})
	var syncErr *Error
	if !errors.As(err, &syncErr) || syncErr.Code != "sync_panic" || !syncErr.Retryable {
		t.Fatalf("Sync() error = %#v", err)
	}
	if repository.failCalls != 1 {
		t.Fatalf("fail calls = %d, want 1", repository.failCalls)
	}
}

type syncRepositoryStub struct {
	sequence     *[]string
	replaced     MirrorBatch
	replaceCalls int
	failCalls    int
	startErr     error
}

func (r *syncRepositoryStub) StartSyncRun(context.Context, SyncRequest) (string, error) {
	*r.sequence = append(*r.sequence, "start-run")
	return "lease-1", r.startErr
}
func (r *syncRepositoryStub) HeartbeatSyncRun(context.Context, SyncRequest) error {
	*r.sequence = append(*r.sequence, "heartbeat")
	return nil
}
func (r *syncRepositoryStub) ReplaceMirror(_ context.Context, _ SyncRequest, batch MirrorBatch) error {
	*r.sequence = append(*r.sequence, "replace-mirror")
	r.replaced = batch
	r.replaceCalls++
	return nil
}
func (r *syncRepositoryStub) FailSyncRun(context.Context, SyncRequest, string, string) error {
	r.failCalls++
	return nil
}

type accessTokenStub struct{ sequence *[]string }

func (s accessTokenStub) AccessToken(context.Context, string, string) (string, error) {
	*s.sequence = append(*s.sequence, "access-token")
	return "access-token", nil
}

type accessTokenPanicStub struct{}

func (accessTokenPanicStub) AccessToken(context.Context, string, string) (string, error) {
	panic("test panic")
}

type storeResolverStub struct{}

func (storeResolverStub) ResolveSyncTarget(context.Context, string, string) (SyncTarget, error) {
	return SyncTarget{Domain: "test.myshopify.com", APIVersion: "2026-07"}, nil
}

type bulkClientStub struct {
	sequence     *[]string
	failProducts bool
}

func (s *bulkClientStub) StartBulkQuery(_ context.Context, _, _, _ string, query string) (shopify.BulkOperation, error) {
	if strings.Contains(query, "products {") {
		*s.sequence = append(*s.sequence, "start-products")
		return shopify.BulkOperation{ID: "products", Status: shopify.BulkStatusCreated}, nil
	}
	*s.sequence = append(*s.sequence, "start-collections")
	return shopify.BulkOperation{ID: "collections", Status: shopify.BulkStatusCreated}, nil
}
func (s *bulkClientStub) GetBulkOperation(_ context.Context, _, _, _, operationID string) (shopify.BulkOperation, error) {
	*s.sequence = append(*s.sequence, "poll-"+operationID)
	if operationID == "products" && s.failProducts {
		return shopify.BulkOperation{ID: operationID, Status: shopify.BulkStatusFailed, ErrorCode: "INTERNAL_SERVER_ERROR"}, nil
	}
	return shopify.BulkOperation{ID: operationID, Status: shopify.BulkStatusCompleted, URL: "https://download/" + operationID}, nil
}
func (s *bulkClientStub) DownloadJSONL(_ context.Context, downloadURL string) (io.ReadCloser, error) {
	kind := strings.TrimPrefix(downloadURL, "https://download/")
	*s.sequence = append(*s.sequence, "download-"+kind)
	if kind == "products" {
		return io.NopCloser(strings.NewReader("{\"id\":\"gid://shopify/Product/1\",\"handle\":\"shirt\",\"title\":\"Shirt\",\"status\":\"ACTIVE\"}\n{\"id\":\"gid://shopify/ProductVariant/1\",\"__parentId\":\"gid://shopify/Product/1\",\"title\":\"Default\",\"price\":\"10.00\"}")), nil
	}
	return io.NopCloser(strings.NewReader("{\"id\":\"gid://shopify/Collection/1\",\"handle\":\"all\",\"title\":\"All\"}")), nil
}
func (s *bulkClientStub) GraphQL(_ context.Context, _, _, _, query string, _ any, target any) error {
	*s.sequence = append(*s.sequence, "list-themes")
	response := target.(*themesQueryResponse)
	response.Themes.Nodes = []themeNode{{ID: "gid://shopify/OnlineStoreTheme/1", Name: "Dawn", Role: "MAIN"}}
	return nil
}
