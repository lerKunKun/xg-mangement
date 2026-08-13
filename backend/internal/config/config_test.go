package config_test

import (
	"strings"
	"testing"

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
