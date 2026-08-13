package integrations_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xg-management/platform/backend/internal/config"
	"github.com/xg-management/platform/backend/internal/integrations"
)

func TestCatalogReportsConfigurationWithoutExposingSecrets(t *testing.T) {
	cfg := config.Config{
		Shopify: config.ShopifyConfig{OAuthProviderConfig: config.OAuthProviderConfig{
			ClientID:     "shopify-client",
			ClientSecret: "shopify-secret",
		}},
		DingTalk: config.OAuthProviderConfig{ClientID: "incomplete"},
		ObjectStorage: config.ObjectStorageConfig{
			Endpoint:        "https://account.r2.cloudflarestorage.com",
			Bucket:          "assets",
			AccessKeyID:     "access-key",
			SecretAccessKey: "secret-key",
		},
	}

	statuses := integrations.Catalog(cfg)
	if len(statuses) != 5 {
		t.Fatalf("len(Catalog()) = %d, want 5", len(statuses))
	}
	configured := make(map[integrations.Provider]bool, len(statuses))
	for _, status := range statuses {
		configured[status.Provider] = status.Configured
	}
	if !configured[integrations.ProviderShopify] || !configured[integrations.ProviderR2] {
		t.Fatalf("configured providers = %#v, want Shopify and R2 configured", configured)
	}
	if configured[integrations.ProviderDingTalk] || configured[integrations.ProviderMetaAds] || configured[integrations.ProviderGoogleAds] {
		t.Fatalf("configured providers = %#v, incomplete providers must be false", configured)
	}
	encoded, err := json.Marshal(statuses)
	if err != nil {
		t.Fatalf("marshal statuses: %v", err)
	}
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "access-key") {
		t.Fatalf("Catalog() JSON exposes credentials: %s", encoded)
	}
}
