package jobs

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	TypeShopifyStoreSyncRequested      = "shopify.store.sync.requested"
	TypeShopifyCatalogPublishRequested = "shopify.catalog.publish.requested"
	TypeReportAggregateRequested       = "report.aggregate.requested"
)

type Envelope struct {
	Version        int             `json:"version"`
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	OrganizationID string          `json:"organization_id"`
	OccurredAt     time.Time       `json:"occurred_at"`
	Payload        json.RawMessage `json:"payload"`
}

func (e Envelope) Validate() error {
	if e.Version != 1 {
		return fmt.Errorf("unsupported version %d", e.Version)
	}
	if e.ID == "" {
		return fmt.Errorf("id is required")
	}
	if e.OrganizationID == "" {
		return fmt.Errorf("organization_id is required")
	}
	switch e.Type {
	case TypeShopifyStoreSyncRequested, TypeShopifyCatalogPublishRequested, TypeReportAggregateRequested:
	default:
		return fmt.Errorf("unsupported type %q", e.Type)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	if len(e.Payload) == 0 || !json.Valid(e.Payload) {
		return fmt.Errorf("payload must be valid JSON")
	}
	return nil
}
