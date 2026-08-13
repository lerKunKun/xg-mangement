package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxProviderResponseBytes = 4 << 20

type ProviderError struct {
	StatusCode int
	Code       string
	Message    string
	Retryable  bool
	RetryAfter time.Duration
}

func (e *ProviderError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "Shopify request failed"
	}
	return fmt.Sprintf("Shopify %s: %s", e.Code, message)
}

type graphQLError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

func (c *Client) GraphQL(ctx context.Context, domain, accessToken, apiVersion, query string, variables any, target any) error {
	normalized, err := NormalizeShopDomain(domain)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("encode Shopify GraphQL request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(normalized, apiVersion), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create Shopify GraphQL request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Shopify-Access-Token", accessToken)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return &ProviderError{Code: "transport_error", Message: limitProviderMessage(err.Error()), Retryable: true}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProviderResponseBytes))
	if err != nil {
		return &ProviderError{StatusCode: response.StatusCode, Code: "response_read_failed", Message: limitProviderMessage(err.Error()), Retryable: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return providerHTTPError(response, body)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &ProviderError{StatusCode: response.StatusCode, Code: "invalid_response", Message: "Shopify returned invalid JSON"}
	}
	if len(envelope.Errors) > 0 {
		code := envelope.Errors[0].Extensions.Code
		if code == "" {
			code = "graphql_error"
		}
		return &ProviderError{StatusCode: response.StatusCode, Code: code, Message: limitProviderMessage(envelope.Errors[0].Message), Retryable: code == "THROTTLED"}
	}
	if target == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return &ProviderError{StatusCode: response.StatusCode, Code: "invalid_data", Message: "Shopify returned an unexpected GraphQL shape"}
	}
	return nil
}

func providerHTTPError(response *http.Response, body []byte) error {
	code := "http_error"
	message := strings.TrimSpace(string(body))
	var providerBody struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &providerBody) == nil {
		if providerBody.Error != "" {
			code = providerBody.Error
		}
		if providerBody.ErrorDescription != "" {
			message = providerBody.ErrorDescription
		}
	}
	retryable := response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
	return &ProviderError{
		StatusCode: response.StatusCode,
		Code:       code,
		Message:    limitProviderMessage(message),
		Retryable:  retryable,
		RetryAfter: parseRetryAfter(response.Header.Get("Retry-After")),
	}
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func limitProviderMessage(value string) string {
	const limit = 512
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
