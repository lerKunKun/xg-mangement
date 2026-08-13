package integrations

import "github.com/xg-management/platform/backend/internal/config"

type Provider string

const (
	ProviderShopify   Provider = "shopify"
	ProviderDingTalk  Provider = "dingtalk"
	ProviderR2        Provider = "cloudflare_r2"
	ProviderMetaAds   Provider = "meta_ads"
	ProviderGoogleAds Provider = "google_ads"
)

type Status struct {
	Provider   Provider `json:"provider"`
	Configured bool     `json:"configured"`
	Status     string   `json:"status"`
	Capability string   `json:"capability"`
}

func Catalog(cfg config.Config) []Status {
	return []Status{
		newStatus(ProviderShopify,
			cfg.Shopify.ClientID != "" && cfg.Shopify.ClientSecret != "",
			"Store authorization, products, orders and webhooks"),
		newStatus(ProviderDingTalk,
			cfg.DingTalk.ClientID != "" && cfg.DingTalk.ClientSecret != "",
			"Organization SSO and approval workflows"),
		newStatus(ProviderR2,
			cfg.ObjectStorage.Endpoint != "" && cfg.ObjectStorage.Bucket != "" && cfg.ObjectStorage.AccessKeyID != "" && cfg.ObjectStorage.SecretAccessKey != "",
			"Product images and reusable site assets"),
		newStatus(ProviderMetaAds,
			cfg.MetaAds.ClientID != "" && cfg.MetaAds.ClientSecret != "",
			"Business portfolios and advertising accounts"),
		newStatus(ProviderGoogleAds,
			cfg.GoogleAds.ClientID != "" && cfg.GoogleAds.ClientSecret != "" && cfg.GoogleAds.DeveloperToken != "",
			"Manager accounts and advertising reports"),
	}
}

func newStatus(provider Provider, configured bool, capability string) Status {
	state := "not_configured"
	if configured {
		state = "ready_for_authorization"
	}
	return Status{Provider: provider, Configured: configured, Status: state, Capability: capability}
}
