package httpapi

import (
	"reflect"
	"testing"
)

func TestNormalizeShopifyScopes(t *testing.T) {
	got := normalizeScopes([]string{" read_products ", "read_themes", "read_products", ""})
	if len(got) != 2 || got[0] != "read_products" || got[1] != "read_themes" {
		t.Fatalf("normalizeScopes() = %#v", got)
	}
	if !scopeListContains(got, requiredShopifyScopes) {
		t.Fatal("required Shopify scopes were not recognized")
	}
	if scopeListContains([]string{"read_products"}, requiredShopifyScopes) {
		t.Fatal("missing read_themes scope was accepted")
	}
}

func TestPhaseOneShopifyScopesAreReadOnly(t *testing.T) {
	configured := phaseOneShopifyScopes()
	configured[0] = "write_themes"
	got := phaseOneShopifyScopes()
	if len(got) != 2 || got[0] != "read_products" || got[1] != "read_themes" {
		t.Fatalf("fixed scopes = %#v", got)
	}
	if sameScopes([]string{"read_products", "read_themes", "write_themes"}, requiredShopifyScopes) {
		t.Fatal("stored overprivileged scopes were accepted")
	}
}

func TestShopifyConfigUsesCanonicalCallbackInsteadOfStoreDomain(t *testing.T) {
	got := normalizeShopifyPublicConfig(shopifyPublicConfig{
		ClientID:    "  client-id  ",
		RedirectURI: "jaxdevstore.myshopify.com",
		Scopes:      []string{"write_themes"},
		APIVersion:  "unstable",
	}, "http://localhost:3001/")

	if got.ClientID != "client-id" {
		t.Fatalf("client ID = %q", got.ClientID)
	}
	if got.RedirectURI != "http://localhost:3001/backend/integrations/shopify/callback" {
		t.Fatalf("redirect URI = %q", got.RedirectURI)
	}
	if !reflect.DeepEqual(got.Scopes, []string{"read_products", "read_themes"}) {
		t.Fatalf("scopes = %#v", got.Scopes)
	}
	if got.APIVersion != "2026-07" {
		t.Fatalf("API version = %q", got.APIVersion)
	}
}

func TestDingTalkConfigUsesOfficialFieldsAndFixedOAuthValues(t *testing.T) {
	got := normalizeDingTalkPublicConfig(dingTalkPublicConfig{
		ClientID:            "  ding-client-id  ",
		CorpID:              "  ding-corp-id  ",
		AgentID:             "  123456  ",
		ApprovalProcessCode: "  PROC-001  ",
		RedirectURI:         "https://wrong.example/callback",
		Scopes:              []string{"profile", "openid"},
	}, "https://console.example.com/")

	if got.ClientID != "ding-client-id" || got.CorpID != "ding-corp-id" || got.AgentID != "123456" || got.ApprovalProcessCode != "PROC-001" {
		t.Fatalf("normalized DingTalk config = %#v", got)
	}
	if got.RedirectURI != "https://console.example.com/backend/integrations/dingtalk/callback" {
		t.Fatalf("redirect URI = %q", got.RedirectURI)
	}
	if !reflect.DeepEqual(got.Scopes, []string{"openid", "corpid"}) {
		t.Fatalf("scopes = %#v", got.Scopes)
	}
}
