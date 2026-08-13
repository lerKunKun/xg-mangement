package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/jobs"
)

func TestOutboxBackoffIsBounded(t *testing.T) {
	if got := outboxBackoff(1); got != time.Second {
		t.Fatalf("first backoff = %v", got)
	}
	if got := outboxBackoff(20); got != 5*time.Minute {
		t.Fatalf("bounded backoff = %v", got)
	}
}

func TestDecodeOutboxEnvelope(t *testing.T) {
	payload, _ := json.Marshal(jobs.Envelope{Version: 1, ID: "job-1", Type: jobs.TypeShopifyStoreSyncRequested, OrganizationID: "org-1", OccurredAt: time.Now(), Payload: json.RawMessage(`{"store_id":"store-1","run_id":"run-1"}`)})
	envelope, err := decodeOutboxEnvelope(payload)
	if err != nil || envelope.ID != "job-1" {
		t.Fatalf("envelope = %#v, error = %v", envelope, err)
	}
}
