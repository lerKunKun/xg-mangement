package shopifysync

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeProductsJSONLConnectsVariantToParent(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"gid://shopify/Product/1","handle":"shirt","title":"Shirt","status":"ACTIVE","vendor":"XG","productType":"Apparel","updatedAt":"2026-08-13T00:00:00Z"}`,
		`{"id":"gid://shopify/ProductVariant/2","__parentId":"gid://shopify/Product/1","sku":"SHIRT-S","title":"Small","price":"29.00","inventoryQuantity":7,"updatedAt":"2026-08-13T00:01:00Z"}`,
	}, "\n")

	products, variants, err := DecodeProductsJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeProductsJSONL() error = %v", err)
	}
	if len(products) != 1 || products[0].Handle != "shirt" {
		t.Fatalf("products = %#v", products)
	}
	if len(variants) != 1 || variants[0].ProductShopifyGID != products[0].ShopifyGID {
		t.Fatalf("variants = %#v", variants)
	}
	if variants[0].InventoryQuantity == nil || *variants[0].InventoryQuantity != 7 {
		t.Fatalf("inventory = %#v", variants[0].InventoryQuantity)
	}
}

func TestDecodeProductsJSONLRejectsMissingParent(t *testing.T) {
	_, _, err := DecodeProductsJSONL(strings.NewReader(`{"id":"gid://shopify/ProductVariant/2","__parentId":"gid://shopify/Product/missing","title":"Small"}`))
	var syncErr *Error
	if !errors.As(err, &syncErr) || syncErr.Code != "bulk_parent_missing" {
		t.Fatalf("error = %#v", err)
	}
}

func TestDecodeJSONLRejectsLineLargerThanLimit(t *testing.T) {
	line := `{"id":"gid://shopify/Product/1","title":"` + strings.Repeat("x", maxBulkLineBytes) + `"}`
	_, _, err := DecodeProductsJSONL(strings.NewReader(line))
	var syncErr *Error
	if !errors.As(err, &syncErr) || syncErr.Code != "bulk_line_too_large" {
		t.Fatalf("error = %#v", err)
	}
}

func TestDecodeCollectionsJSONL(t *testing.T) {
	input := `{"id":"gid://shopify/Collection/1","handle":"summer","title":"Summer","productsCount":{"count":12},"updatedAt":"2026-08-13T00:00:00Z"}`
	collections, err := DecodeCollectionsJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeCollectionsJSONL() error = %v", err)
	}
	if len(collections) != 1 || collections[0].ProductsCount != 12 {
		t.Fatalf("collections = %#v", collections)
	}
}
