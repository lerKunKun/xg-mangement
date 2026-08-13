package shopify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"testing"
)

func TestNormalizeShopDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "Example-Store.myshopify.com", want: "example-store.myshopify.com", ok: true},
		{input: "https://example-store.myshopify.com", ok: false},
		{input: "example.com", ok: false},
		{input: "evil.myshopify.com.attacker.test", ok: false},
		{input: "-bad.myshopify.com", ok: false},
	}
	for _, tt := range tests {
		got, err := NormalizeShopDomain(tt.input)
		if tt.ok && err != nil {
			t.Fatalf("NormalizeShopDomain(%q): %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("NormalizeShopDomain(%q) succeeded", tt.input)
		}
		if got != tt.want {
			t.Fatalf("NormalizeShopDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestVerifyCallbackHMAC(t *testing.T) {
	query := url.Values{
		"code":      {"authorization-code"},
		"shop":      {"demo.myshopify.com"},
		"state":     {"nonce"},
		"timestamp": {"1720000000"},
	}
	mac := hmac.New(sha256.New, []byte("client-secret"))
	_, _ = mac.Write([]byte(query.Encode()))
	query.Set("hmac", hex.EncodeToString(mac.Sum(nil)))

	if !VerifyCallbackHMAC(query, "client-secret") {
		t.Fatal("valid callback HMAC rejected")
	}
	query.Set("shop", "attacker.myshopify.com")
	if VerifyCallbackHMAC(query, "client-secret") {
		t.Fatal("tampered callback HMAC accepted")
	}
}

func TestAuthorizationURLUsesOfflineExpiringFlow(t *testing.T) {
	got, err := AuthorizationURL("demo.myshopify.com", OAuthConfig{
		ClientID:    "client-id",
		RedirectURI: "https://app.example.com/api/v1/integrations/shopify/callback",
		Scopes:      []string{"read_products", "write_products"},
	}, "state-value")
	if err != nil {
		t.Fatalf("AuthorizationURL: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if parsed.Host != "demo.myshopify.com" || parsed.Path != "/admin/oauth/authorize" {
		t.Fatalf("authorization endpoint = %s%s", parsed.Host, parsed.Path)
	}
	if parsed.Query().Get("state") != "state-value" {
		t.Fatalf("state = %q", parsed.Query().Get("state"))
	}
	if parsed.Query().Has("grant_options[]") {
		t.Fatal("offline flow must omit grant_options[]")
	}
}
