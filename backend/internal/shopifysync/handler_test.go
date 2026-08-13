package shopifysync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/jobs"
)

func TestHandlerDecodesSyncRequest(t *testing.T) {
	processor := &syncProcessorStub{}
	handler := Handler{Syncer: processor}
	envelope := jobs.Envelope{Version: 1, ID: "run-1", Type: jobs.TypeShopifyStoreSyncRequested, OrganizationID: "org-1", OccurredAt: time.Now(), Payload: json.RawMessage(`{"store_id":"store-1","run_id":"run-1","mode":"full","future_field":true}`)}

	if err := handler.Handle(context.Background(), envelope); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if processor.request.OrganizationID != "org-1" || processor.request.StoreID != "store-1" || processor.request.RunID != "run-1" || processor.request.Mode != SyncModeFull {
		t.Fatalf("request = %#v", processor.request)
	}
}

func TestHandlerRejectsMissingRunIDAsPermanent(t *testing.T) {
	handler := Handler{Syncer: &syncProcessorStub{}}
	envelope := jobs.Envelope{Version: 1, ID: "job-1", Type: jobs.TypeShopifyStoreSyncRequested, OrganizationID: "org-1", OccurredAt: time.Now(), Payload: json.RawMessage(`{"store_id":"store-1"}`)}

	err := handler.Handle(context.Background(), envelope)
	var handlerErr *HandlerError
	if !errors.As(err, &handlerErr) || handlerErr.Retryable {
		t.Fatalf("error = %#v, want permanent handler error", err)
	}
}

type syncProcessorStub struct{ request SyncRequest }

func (s *syncProcessorStub) Sync(_ context.Context, request SyncRequest) error {
	s.request = request
	return nil
}
