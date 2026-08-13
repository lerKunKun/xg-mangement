package config_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/config"
)

func TestLoadRejectsDevelopmentLoginInProduction(t *testing.T) {
	lookup := envLookup(map[string]string{
		"APP_ENV":                "production",
		"AUTH_DEV_LOGIN_ENABLED": "true",
		"SESSION_SECRET":         strings.Repeat("s", 32),
	})

	_, err := config.Load(lookup)
	if err == nil || !strings.Contains(err.Error(), "development login") {
		t.Fatalf("Load() error = %v, want development login safety error", err)
	}
}

func TestLoadShopifySyncRuntimeConfiguration(t *testing.T) {
	cfg, err := config.Load(envLookup(map[string]string{
		"SHOPIFY_SYNC_POLL_INTERVAL": "2s",
		"SHOPIFY_SYNC_TIMEOUT":       "10m",
		"SHOPIFY_SYNC_MAX_ATTEMPTS":  "4",
		"RABBITMQ_RETRY_DELAY":       "45s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ShopifySync.PollInterval != 2*time.Second || cfg.ShopifySync.Timeout != 10*time.Minute || cfg.ShopifySync.MaxAttempts != 4 || cfg.RabbitMQRetryDelay != 45*time.Second {
		t.Fatalf("runtime config = %#v, retry = %v", cfg.ShopifySync, cfg.RabbitMQRetryDelay)
	}
}

func TestLoadRejectsNonPositiveShopifyAttempts(t *testing.T) {
	_, err := config.Load(envLookup(map[string]string{"SHOPIFY_SYNC_MAX_ATTEMPTS": "0"}))
	if err == nil || !strings.Contains(err.Error(), "SHOPIFY_SYNC_MAX_ATTEMPTS") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsShortProductionSessionSecret(t *testing.T) {
	lookup := envLookup(map[string]string{
		"APP_ENV":        "production",
		"SESSION_SECRET": "too-short",
	})

	_, err := config.Load(lookup)
	if err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("Load() error = %v, want SESSION_SECRET safety error", err)
	}
}

func TestLoadRequiresExplicitCredentialKeyInProduction(t *testing.T) {
	_, err := config.Load(envLookup(map[string]string{
		"APP_ENV":        "production",
		"SESSION_SECRET": strings.Repeat("s", 32),
	}))
	if err == nil || !strings.Contains(err.Error(), "CREDENTIAL_ENCRYPTION_KEY") {
		t.Fatalf("Load() error = %v, want credential key safety error", err)
	}
}

func TestLoadRejectsEncodedDevelopmentCredentialKeyInProduction(t *testing.T) {
	encodedDefault := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	_, err := config.Load(envLookup(map[string]string{
		"APP_ENV":                   "production",
		"SESSION_SECRET":            strings.Repeat("s", 32),
		"CREDENTIAL_ENCRYPTION_KEY": encodedDefault,
	}))
	if err == nil || !strings.Contains(err.Error(), "non-development") {
		t.Fatalf("Load() error = %v, want encoded development key rejection", err)
	}
}

func TestLoadRejectsUnsupportedEnvironmentAlias(t *testing.T) {
	_, err := config.Load(envLookup(map[string]string{"APP_ENV": "Production"}))
	if err == nil || !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("Load() error = %v, want strict APP_ENV validation", err)
	}
}

func TestLoadRejectsMalformedCredentialKey(t *testing.T) {
	_, err := config.Load(envLookup(map[string]string{"CREDENTIAL_ENCRYPTION_KEY": "not-an-aes-key"}))
	if err == nil || !strings.Contains(err.Error(), "CREDENTIAL_ENCRYPTION_KEY") {
		t.Fatalf("Load() error = %v, want credential key validation error", err)
	}
}

func TestLoadRejectsUnsupportedShopifyAPIVersion(t *testing.T) {
	_, err := config.Load(envLookup(map[string]string{"SHOPIFY_API_VERSION": "unstable"}))
	if err == nil || !strings.Contains(err.Error(), "SHOPIFY_API_VERSION") {
		t.Fatalf("Load() error = %v, want fixed Shopify API version error", err)
	}
}

func TestLoadAcceptsExplicitProductionCredentialKey(t *testing.T) {
	_, err := config.Load(envLookup(map[string]string{
		"APP_ENV":                   "production",
		"SESSION_SECRET":            strings.Repeat("s", 32),
		"CREDENTIAL_ENCRYPTION_KEY": strings.Repeat("k", 32),
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadProvidesSafeDevelopmentDefaults(t *testing.T) {
	cfg, err := config.Load(envLookup(map[string]string{}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("HTTPAddress = %q, want :8080", cfg.HTTPAddress)
	}
	if cfg.Auth.DevLoginEnabled {
		t.Fatal("DevLoginEnabled = true, want opt-in false")
	}
}

func envLookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
