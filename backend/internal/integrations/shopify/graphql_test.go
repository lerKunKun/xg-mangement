package shopify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGraphQLSendsVersionedAuthenticatedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Shopify-Access-Token"); got != "access-secret" {
			t.Fatalf("access token header = %q", got)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		_, _ = w.Write([]byte(`{"data":{"shop":{"id":"gid://shopify/Shop/1"}}}`))
	}))
	defer server.Close()

	client := NewClientWithEndpoint(server.Client(), func(domain, apiVersion string) string {
		if domain != "test.myshopify.com" || apiVersion != "2026-07" {
			t.Fatalf("endpoint arguments = %q, %q", domain, apiVersion)
		}
		return server.URL
	})
	var target struct {
		Shop struct {
			ID string `json:"id"`
		} `json:"shop"`
	}
	err := client.GraphQL(context.Background(), "test.myshopify.com", "access-secret", "2026-07", "query { shop { id } }", nil, &target)
	if err != nil {
		t.Fatalf("GraphQL() error = %v", err)
	}
	if target.Shop.ID != "gid://shopify/Shop/1" {
		t.Fatalf("shop id = %q", target.Shop.ID)
	}
}

func TestGraphQLReturnsTypedRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"errors":"throttled"}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	err := client.GraphQL(context.Background(), "test.myshopify.com", "secret", "2026-07", "query { shop { id } }", nil, &struct{}{})
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || !providerErr.Retryable || providerErr.RetryAfter != 2*time.Second {
		t.Fatalf("error = %#v, want retryable ProviderError with two second delay", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked access token: %v", err)
	}
}

func TestGraphQLReturnsTopLevelErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Access denied","extensions":{"code":"ACCESS_DENIED"}}]}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	err := client.GraphQL(context.Background(), "test.myshopify.com", "secret", "2026-07", "query { shop { id } }", nil, &struct{}{})
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) || providerErr.Code != "ACCESS_DENIED" || providerErr.Retryable {
		t.Fatalf("error = %#v, want permanent ACCESS_DENIED", err)
	}
}
