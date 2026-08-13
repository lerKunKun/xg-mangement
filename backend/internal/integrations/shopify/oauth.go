package shopify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var shopDomainPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.myshopify\.com$`)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	APIVersion   string
}

func NormalizeShopDomain(value string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(value))
	if !shopDomainPattern.MatchString(domain) {
		return "", fmt.Errorf("invalid Shopify shop domain")
	}
	return domain, nil
}

func AuthorizationURL(shop string, cfg OAuthConfig, state string) (string, error) {
	domain, err := NormalizeShopDomain(shop)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.RedirectURI) == "" || strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("Shopify OAuth configuration is incomplete")
	}
	endpoint := url.URL{Scheme: "https", Host: domain, Path: "/admin/oauth/authorize"}
	query := endpoint.Query()
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.RedirectURI)
	query.Set("scope", strings.Join(cfg.Scopes, ","))
	query.Set("state", state)
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func VerifyCallbackHMAC(query url.Values, clientSecret string) bool {
	provided, err := hex.DecodeString(query.Get("hmac"))
	if err != nil || len(provided) == 0 || clientSecret == "" {
		return false
	}
	unsigned := cloneValues(query)
	unsigned.Del("hmac")
	mac := hmac.New(sha256.New, []byte(clientSecret))
	_, _ = mac.Write([]byte(unsigned.Encode()))
	return hmac.Equal(provided, mac.Sum(nil))
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, entries := range values {
		cloned[key] = append([]string(nil), entries...)
	}
	return cloned
}
