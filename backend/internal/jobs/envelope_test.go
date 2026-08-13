package jobs_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/jobs"
)

func TestEnvelopeValidate(t *testing.T) {
	valid := jobs.Envelope{
		Version:        1,
		ID:             "job-1",
		Type:           jobs.TypeShopifyStoreSyncRequested,
		OrganizationID: "org-1",
		OccurredAt:     time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		Payload:        json.RawMessage(`{"store_id":"store-1"}`),
	}

	tests := []struct {
		name        string
		mutate      func(*jobs.Envelope)
		wantErrPart string
	}{
		{name: "valid envelope", mutate: func(*jobs.Envelope) {}},
		{name: "missing ID", mutate: func(e *jobs.Envelope) { e.ID = "" }, wantErrPart: "id"},
		{name: "missing organization", mutate: func(e *jobs.Envelope) { e.OrganizationID = "" }, wantErrPart: "organization_id"},
		{name: "unknown type", mutate: func(e *jobs.Envelope) { e.Type = "unknown" }, wantErrPart: "type"},
		{name: "unsupported version", mutate: func(e *jobs.Envelope) { e.Version = 2 }, wantErrPart: "version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := valid
			tt.mutate(&envelope)
			err := envelope.Validate()
			if tt.wantErrPart == "" && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
			if tt.wantErrPart != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrPart)) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tt.wantErrPart)
			}
		})
	}
}
