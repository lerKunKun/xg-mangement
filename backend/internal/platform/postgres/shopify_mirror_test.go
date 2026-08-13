package postgres

import (
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/shopifysync"
)

func TestNormalizeMirrorBatchKeepsNewestResourceVersion(t *testing.T) {
	oldTime := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	batch := shopifysync.MirrorBatch{
		Products: []shopifysync.Product{
			{ShopifyGID: "gid://shopify/Product/1", Title: "Old", SourceUpdatedAt: &oldTime},
			{ShopifyGID: "gid://shopify/Product/1", Title: "New", SourceUpdatedAt: &newTime},
		},
		Variants: []shopifysync.Variant{
			{ShopifyGID: "gid://shopify/ProductVariant/1", ProductShopifyGID: "gid://shopify/Product/missing"},
		},
	}

	normalized, err := normalizeMirrorBatch(batch)
	if err == nil {
		t.Fatal("normalizeMirrorBatch() error = nil, want missing parent error")
	}
	batch.Variants[0].ProductShopifyGID = "gid://shopify/Product/1"
	normalized, err = normalizeMirrorBatch(batch)
	if err != nil {
		t.Fatalf("normalizeMirrorBatch() error = %v", err)
	}
	if len(normalized.Products) != 1 || normalized.Products[0].Title != "New" {
		t.Fatalf("products = %#v, want newest duplicate", normalized.Products)
	}
}
