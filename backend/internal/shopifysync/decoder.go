package shopifysync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const maxBulkLineBytes = 4 << 20

type Error struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

type bulkResource struct {
	ID                string `json:"id"`
	ParentID          string `json:"__parentId"`
	Handle            string `json:"handle"`
	Title             string `json:"title"`
	Status            string `json:"status"`
	Vendor            string `json:"vendor"`
	ProductType       string `json:"productType"`
	SKU               string `json:"sku"`
	Price             string `json:"price"`
	CompareAtPrice    string `json:"compareAtPrice"`
	InventoryQuantity *int   `json:"inventoryQuantity"`
	UpdatedAt         string `json:"updatedAt"`
	ProductsCount     struct {
		Count int `json:"count"`
	} `json:"productsCount"`
}

func DecodeProductsJSONL(reader io.Reader) ([]Product, []Variant, error) {
	products := make([]Product, 0)
	variants := make([]Variant, 0)
	productIDs := make(map[string]struct{})
	err := scanBulkLines(reader, func(line json.RawMessage, resource bulkResource) error {
		updatedAt, err := optionalTime(resource.UpdatedAt)
		if err != nil {
			return &Error{Code: "bulk_invalid_timestamp", Message: err.Error()}
		}
		switch {
		case strings.HasPrefix(resource.ID, "gid://shopify/ProductVariant/") || resource.ParentID != "":
			variants = append(variants, Variant{
				ShopifyGID:        resource.ID,
				ProductShopifyGID: resource.ParentID,
				SKU:               resource.SKU,
				Title:             resource.Title,
				Price:             resource.Price,
				CompareAtPrice:    resource.CompareAtPrice,
				InventoryQuantity: resource.InventoryQuantity,
				Payload:           append(json.RawMessage(nil), line...),
				SourceUpdatedAt:   updatedAt,
			})
		case strings.HasPrefix(resource.ID, "gid://shopify/Product/"):
			products = append(products, Product{
				ShopifyGID:      resource.ID,
				Handle:          resource.Handle,
				Title:           resource.Title,
				Status:          resource.Status,
				Vendor:          resource.Vendor,
				ProductType:     resource.ProductType,
				Payload:         append(json.RawMessage(nil), line...),
				SourceUpdatedAt: updatedAt,
			})
			productIDs[resource.ID] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	for _, variant := range variants {
		if _, exists := productIDs[variant.ProductShopifyGID]; !exists {
			return nil, nil, &Error{Code: "bulk_parent_missing", Message: fmt.Sprintf("variant %s references missing product %s", variant.ShopifyGID, variant.ProductShopifyGID)}
		}
	}
	return products, variants, nil
}

func DecodeCollectionsJSONL(reader io.Reader) ([]Collection, error) {
	collections := make([]Collection, 0)
	err := scanBulkLines(reader, func(line json.RawMessage, resource bulkResource) error {
		if !strings.HasPrefix(resource.ID, "gid://shopify/Collection/") {
			return nil
		}
		updatedAt, err := optionalTime(resource.UpdatedAt)
		if err != nil {
			return &Error{Code: "bulk_invalid_timestamp", Message: err.Error()}
		}
		collections = append(collections, Collection{
			ShopifyGID:      resource.ID,
			Handle:          resource.Handle,
			Title:           resource.Title,
			ProductsCount:   resource.ProductsCount.Count,
			Payload:         append(json.RawMessage(nil), line...),
			SourceUpdatedAt: updatedAt,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return collections, nil
}

func scanBulkLines(reader io.Reader, visit func(json.RawMessage, bulkResource) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), maxBulkLineBytes)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := append(json.RawMessage(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var resource bulkResource
		if err := json.Unmarshal(line, &resource); err != nil {
			return &Error{Code: "bulk_invalid_json", Message: fmt.Sprintf("line %d is invalid JSON", lineNumber)}
		}
		if resource.ID == "" {
			continue
		}
		if err := visit(line, resource); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			return &Error{Code: "bulk_line_too_large", Message: "Shopify bulk JSONL line exceeds 4 MiB"}
		}
		return &Error{Code: "bulk_read_failed", Message: err.Error(), Retryable: true}
	}
	return nil
}

func optionalTime(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, fmt.Errorf("invalid Shopify timestamp")
	}
	return &parsed, nil
}
