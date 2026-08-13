package httpapi

import "testing"

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
