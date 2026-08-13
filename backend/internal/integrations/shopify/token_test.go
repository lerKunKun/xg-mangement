package shopify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRefreshRotatesOfflineToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{
			"grant_type":    "refresh_token",
			"client_id":     "client-id",
			"client_secret": "client-secret",
			"refresh_token": "refresh-old",
		}
		for key, value := range want {
			if form.Get(key) != value {
				t.Fatalf("%s = %q, want %q", key, form.Get(key), value)
			}
		}
		_, _ = w.Write([]byte(`{"access_token":"access-new","expires_in":3600,"refresh_token":"refresh-new","refresh_token_expires_in":7776000,"scope":"read_products"}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	token, err := client.Refresh(context.Background(), "test.myshopify.com", "client-id", "client-secret", "refresh-old")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if token.AccessToken != "access-new" || token.RefreshToken != "refresh-new" {
		t.Fatalf("token = %#v, want rotated tokens", token)
	}
}

func TestRefreshMarksInvalidRefreshTokenAsReauthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_request","error_description":"This request requires an active refresh_token"}`))
	}))
	defer server.Close()
	client := NewClientWithEndpoint(server.Client(), func(string, string) string { return server.URL })

	_, err := client.Refresh(context.Background(), "test.myshopify.com", "client-id", "client-secret", "refresh-old")
	if !IsReauthorizationRequired(err) {
		t.Fatalf("Refresh() error = %#v, want reauthorization required", err)
	}
}
