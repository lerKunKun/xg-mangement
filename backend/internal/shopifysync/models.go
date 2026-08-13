package shopifysync

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrSyncAlreadyRunning = errors.New("Shopify sync is already running")

type SyncMode string

const (
	SyncModeFull        SyncMode = "full"
	SyncModeIncremental SyncMode = "incremental"
)

type SyncStatus string

const (
	SyncStatusQueued    SyncStatus = "queued"
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

type SyncRequest struct {
	OrganizationID string   `json:"organization_id"`
	StoreID        string   `json:"store_id"`
	RunID          string   `json:"run_id"`
	Mode           SyncMode `json:"mode,omitempty"`
}

func (r SyncRequest) Validate() error {
	if strings.TrimSpace(r.OrganizationID) == "" {
		return fmt.Errorf("organization_id is required")
	}
	if strings.TrimSpace(r.StoreID) == "" {
		return fmt.Errorf("store_id is required")
	}
	if strings.TrimSpace(r.RunID) == "" {
		return fmt.Errorf("run_id is required")
	}
	if r.Mode != "" && r.Mode != SyncModeFull && r.Mode != SyncModeIncremental {
		return fmt.Errorf("unsupported sync mode %q", r.Mode)
	}
	return nil
}

type ResourceCounts struct {
	Products    int `json:"products"`
	Variants    int `json:"variants"`
	Collections int `json:"collections"`
	Themes      int `json:"themes"`
}

type SyncRun struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organization_id,omitempty"`
	StoreID        string         `json:"store_id"`
	Mode           SyncMode       `json:"mode"`
	Status         SyncStatus     `json:"status"`
	JobID          string         `json:"job_id"`
	Counts         ResourceCounts `json:"counts"`
	ErrorCode      string         `json:"error_code,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type StoreConnection struct {
	OrganizationID       string
	StoreID              string
	AccountID            string
	Domain               string
	Status               string
	EncryptedCredentials []byte
	ExpiresAt            *time.Time
	RefreshExpiresAt     *time.Time
	PublicConfig         json.RawMessage
	EncryptedSecrets     []byte
	LastErrorCode        string
}

// CredentialUpdate is applied while the store credential row is still locked.
// Empty credential bytes preserve the existing ciphertext; an empty error code
// clears a previous provider error when Status is connected.
type CredentialUpdate struct {
	Changed              bool
	EncryptedCredentials []byte
	ExpiresAt            *time.Time
	RefreshExpiresAt     *time.Time
	Status               string
	LastErrorCode        string
}

type Product struct {
	ShopifyGID      string          `json:"shopify_gid"`
	Handle          string          `json:"handle"`
	Title           string          `json:"title"`
	Status          string          `json:"status"`
	Vendor          string          `json:"vendor,omitempty"`
	ProductType     string          `json:"product_type,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	SourceUpdatedAt *time.Time      `json:"source_updated_at,omitempty"`
}

type Variant struct {
	ShopifyGID        string          `json:"shopify_gid"`
	ProductShopifyGID string          `json:"product_shopify_gid"`
	SKU               string          `json:"sku,omitempty"`
	Title             string          `json:"title"`
	Price             string          `json:"price,omitempty"`
	CompareAtPrice    string          `json:"compare_at_price,omitempty"`
	InventoryQuantity *int            `json:"inventory_quantity,omitempty"`
	Payload           json.RawMessage `json:"payload,omitempty"`
	SourceUpdatedAt   *time.Time      `json:"source_updated_at,omitempty"`
}

type Collection struct {
	ShopifyGID      string          `json:"shopify_gid"`
	Handle          string          `json:"handle"`
	Title           string          `json:"title"`
	CollectionType  string          `json:"collection_type,omitempty"`
	ProductsCount   int             `json:"products_count"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	SourceUpdatedAt *time.Time      `json:"source_updated_at,omitempty"`
}

type Theme struct {
	ShopifyGID       string     `json:"shopify_gid"`
	Name             string     `json:"name"`
	Role             string     `json:"role"`
	Processing       bool       `json:"processing"`
	ProcessingFailed bool       `json:"processing_failed"`
	ThemeStoreID     *int64     `json:"theme_store_id,omitempty"`
	SourceReleaseID  *string    `json:"source_release_id,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	SyncedAt         time.Time  `json:"synced_at,omitempty"`
}

type MirrorBatch struct {
	Products    []Product
	Variants    []Variant
	Collections []Collection
	Themes      []Theme
	SyncedAt    time.Time
}

func (b MirrorBatch) Counts() ResourceCounts {
	return ResourceCounts{
		Products:    len(b.Products),
		Variants:    len(b.Variants),
		Collections: len(b.Collections),
		Themes:      len(b.Themes),
	}
}
