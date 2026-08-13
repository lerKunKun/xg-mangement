package config

import (
	"fmt"
	"strconv"
	"strings"
)

type Config struct {
	Environment             string
	HTTPAddress             string
	DatabaseURL             string
	RedisURL                string
	RabbitMQURL             string
	RabbitMQQueue           string
	Auth                    AuthConfig
	WebBaseURL              string
	CredentialEncryptionKey string
	ObjectStorage           ObjectStorageConfig
	DingTalk                OAuthProviderConfig
	Shopify                 ShopifyConfig
	MetaAds                 OAuthProviderConfig
	GoogleAds               GoogleAdsConfig
}

type AuthConfig struct {
	DevLoginEnabled bool
	SessionSecret   string
}

type ObjectStorageConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type ShopifyConfig struct {
	OAuthProviderConfig
	APIVersion string
	Scopes     string
}

type GoogleAdsConfig struct {
	OAuthProviderConfig
	DeveloperToken string
}

func Load(lookup func(string) string) (Config, error) {
	environment := valueOrDefault(lookup("APP_ENV"), "development")
	devLoginEnabled, err := strconv.ParseBool(valueOrDefault(lookup("AUTH_DEV_LOGIN_ENABLED"), "false"))
	if err != nil {
		return Config{}, fmt.Errorf("parse AUTH_DEV_LOGIN_ENABLED: %w", err)
	}
	usePathStyle, err := strconv.ParseBool(valueOrDefault(lookup("OBJECT_STORAGE_PATH_STYLE"), "true"))
	if err != nil {
		return Config{}, fmt.Errorf("parse OBJECT_STORAGE_PATH_STYLE: %w", err)
	}

	cfg := Config{
		Environment:   environment,
		HTTPAddress:   valueOrDefault(lookup("HTTP_ADDRESS"), ":8080"),
		DatabaseURL:   valueOrDefault(lookup("DATABASE_URL"), "postgres://xg:xg@localhost:5432/xg?sslmode=disable"),
		RedisURL:      valueOrDefault(lookup("REDIS_URL"), "redis://localhost:6379/0"),
		RabbitMQURL:   valueOrDefault(lookup("RABBITMQ_URL"), "amqp://xg:xg@localhost:5672/"),
		RabbitMQQueue: valueOrDefault(lookup("RABBITMQ_QUEUE"), "xg.jobs"),
		Auth: AuthConfig{
			DevLoginEnabled: devLoginEnabled,
			SessionSecret:   lookup("SESSION_SECRET"),
		},
		WebBaseURL:              valueOrDefault(lookup("WEB_BASE_URL"), "http://localhost:3001"),
		CredentialEncryptionKey: valueOrDefault(lookup("CREDENTIAL_ENCRYPTION_KEY"), "0123456789abcdef0123456789abcdef"),
		ObjectStorage: ObjectStorageConfig{
			Endpoint:        valueOrDefault(lookup("OBJECT_STORAGE_ENDPOINT"), "http://localhost:9000"),
			Region:          valueOrDefault(lookup("OBJECT_STORAGE_REGION"), "auto"),
			Bucket:          valueOrDefault(lookup("OBJECT_STORAGE_BUCKET"), "xg-assets"),
			AccessKeyID:     valueOrDefault(lookup("OBJECT_STORAGE_ACCESS_KEY_ID"), "minioadmin"),
			SecretAccessKey: valueOrDefault(lookup("OBJECT_STORAGE_SECRET_ACCESS_KEY"), "minioadmin"),
			UsePathStyle:    usePathStyle,
		},
		DingTalk: OAuthProviderConfig{
			ClientID:     lookup("DINGTALK_CLIENT_ID"),
			ClientSecret: lookup("DINGTALK_CLIENT_SECRET"),
			RedirectURI:  lookup("DINGTALK_REDIRECT_URI"),
		},
		Shopify: ShopifyConfig{
			OAuthProviderConfig: OAuthProviderConfig{
				ClientID:     lookup("SHOPIFY_CLIENT_ID"),
				ClientSecret: lookup("SHOPIFY_CLIENT_SECRET"),
				RedirectURI:  lookup("SHOPIFY_REDIRECT_URI"),
			},
			APIVersion: valueOrDefault(lookup("SHOPIFY_API_VERSION"), "2026-07"),
			Scopes:     valueOrDefault(lookup("SHOPIFY_SCOPES"), "read_products,write_products,read_orders"),
		},
		MetaAds: OAuthProviderConfig{
			ClientID:     lookup("META_APP_ID"),
			ClientSecret: lookup("META_APP_SECRET"),
			RedirectURI:  lookup("META_REDIRECT_URI"),
		},
		GoogleAds: GoogleAdsConfig{
			OAuthProviderConfig: OAuthProviderConfig{
				ClientID:     lookup("GOOGLE_ADS_CLIENT_ID"),
				ClientSecret: lookup("GOOGLE_ADS_CLIENT_SECRET"),
				RedirectURI:  lookup("GOOGLE_ADS_REDIRECT_URI"),
			},
			DeveloperToken: lookup("GOOGLE_ADS_DEVELOPER_TOKEN"),
		},
	}

	if environment == "production" {
		if cfg.Auth.DevLoginEnabled {
			return Config{}, fmt.Errorf("development login must be disabled in production")
		}
		if len(cfg.Auth.SessionSecret) < 32 {
			return Config{}, fmt.Errorf("SESSION_SECRET must contain at least 32 characters in production")
		}
		if len(cfg.CredentialEncryptionKey) < 32 {
			return Config{}, fmt.Errorf("CREDENTIAL_ENCRYPTION_KEY must contain an AES-256 key in production")
		}
	}

	return cfg, nil
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
