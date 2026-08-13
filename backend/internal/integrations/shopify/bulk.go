package shopify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type BulkStatus string

const (
	BulkStatusCreated   BulkStatus = "CREATED"
	BulkStatusRunning   BulkStatus = "RUNNING"
	BulkStatusCompleted BulkStatus = "COMPLETED"
	BulkStatusFailed    BulkStatus = "FAILED"
	BulkStatusCanceled  BulkStatus = "CANCELED"
	BulkStatusExpired   BulkStatus = "EXPIRED"
)

type BulkOperation struct {
	ID             string     `json:"id"`
	Status         BulkStatus `json:"status"`
	ErrorCode      string     `json:"errorCode"`
	ObjectCount    string     `json:"objectCount"`
	FileSize       string     `json:"fileSize"`
	URL            string     `json:"url"`
	PartialDataURL string     `json:"partialDataUrl"`
}

type userError struct {
	Field   []string `json:"field"`
	Message string   `json:"message"`
}

func (c *Client) StartBulkQuery(ctx context.Context, domain, accessToken, apiVersion, bulkQuery string) (BulkOperation, error) {
	const mutation = `mutation RunBulkQuery($query: String!) {
  bulkOperationRunQuery(query: $query) {
    bulkOperation { id status }
    userErrors { field message }
  }
}`
	var data struct {
		BulkOperationRunQuery struct {
			BulkOperation *BulkOperation `json:"bulkOperation"`
			UserErrors    []userError    `json:"userErrors"`
		} `json:"bulkOperationRunQuery"`
	}
	if err := c.GraphQL(ctx, domain, accessToken, apiVersion, mutation, map[string]any{"query": bulkQuery}, &data); err != nil {
		return BulkOperation{}, err
	}
	result := data.BulkOperationRunQuery
	if len(result.UserErrors) > 0 {
		return BulkOperation{}, &ProviderError{Code: "bulk_query_invalid", Message: limitProviderMessage(result.UserErrors[0].Message)}
	}
	if result.BulkOperation == nil || result.BulkOperation.ID == "" {
		return BulkOperation{}, &ProviderError{Code: "invalid_response", Message: "Shopify did not return a bulk operation"}
	}
	return *result.BulkOperation, nil
}

func (c *Client) GetBulkOperation(ctx context.Context, domain, accessToken, apiVersion, operationID string) (BulkOperation, error) {
	const query = `query BulkOperation($id: ID!) {
  bulkOperation(id: $id) {
    id status errorCode objectCount fileSize url partialDataUrl
  }
}`
	var data struct {
		BulkOperation *BulkOperation `json:"bulkOperation"`
	}
	if err := c.GraphQL(ctx, domain, accessToken, apiVersion, query, map[string]any{"id": operationID}, &data); err != nil {
		return BulkOperation{}, err
	}
	if data.BulkOperation == nil {
		return BulkOperation{}, &ProviderError{Code: "bulk_operation_not_found", Message: "Shopify bulk operation was not found"}
	}
	return *data.BulkOperation, nil
}

func (c *Client) DownloadJSONL(ctx context.Context, downloadURL string) (io.ReadCloser, error) {
	if !strings.HasPrefix(downloadURL, "https://") && !strings.HasPrefix(downloadURL, "http://") {
		return nil, fmt.Errorf("invalid Shopify bulk download URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create Shopify bulk download request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, &ProviderError{Code: "bulk_download_failed", Message: limitProviderMessage(err.Error()), Retryable: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		return nil, providerHTTPError(response, nil)
	}
	return response.Body, nil
}
