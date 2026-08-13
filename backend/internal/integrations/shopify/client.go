package shopify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Token struct {
	AccessToken           string `json:"access_token"`
	Scope                 string `json:"scope"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
}

type Shop struct {
	ID            string
	Name          string
	CurrencyCode  string
	Timezone      string
	PlanName      string
	PrimaryDomain string
}

type Client struct{ httpClient *http.Client }

func NewClient() *Client { return &Client{httpClient: &http.Client{Timeout: 20 * time.Second}} }

func (c *Client) Exchange(ctx context.Context, shop string, cfg OAuthConfig, code string) (Token, error) {
	domain, err := NormalizeShopDomain(shop)
	if err != nil {
		return Token{}, err
	}
	form := url.Values{"client_id": {cfg.ClientID}, "client_secret": {cfg.ClientSecret}, "code": {code}, "expiring": {"1"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+domain+"/admin/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	var token Token
	if err := c.doJSON(request, &token); err != nil {
		return Token{}, fmt.Errorf("exchange Shopify authorization code: %w", err)
	}
	if token.AccessToken == "" {
		return Token{}, fmt.Errorf("Shopify returned an empty access token")
	}
	return token, nil
}

func (c *Client) Shop(ctx context.Context, domain, accessToken, apiVersion string) (Shop, error) {
	domain, err := NormalizeShopDomain(domain)
	if err != nil {
		return Shop{}, err
	}
	body := strings.NewReader(`{"query":"query { shop { id name currencyCode ianaTimezone plan { displayName } primaryDomain { url } } }"}`)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+domain+"/admin/api/"+apiVersion+"/graphql.json", body)
	if err != nil {
		return Shop{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Shopify-Access-Token", accessToken)
	var response struct {
		Data struct {
			Shop struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				CurrencyCode string `json:"currencyCode"`
				Timezone     string `json:"ianaTimezone"`
				Plan         struct {
					DisplayName string `json:"displayName"`
				} `json:"plan"`
				PrimaryDomain struct {
					URL string `json:"url"`
				} `json:"primaryDomain"`
			} `json:"shop"`
		} `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	if err := c.doJSON(request, &response); err != nil {
		return Shop{}, fmt.Errorf("load Shopify shop: %w", err)
	}
	if len(response.Errors) > 0 && string(response.Errors) != "null" {
		return Shop{}, fmt.Errorf("Shopify GraphQL errors: %s", response.Errors)
	}
	return Shop{ID: response.Data.Shop.ID, Name: response.Data.Shop.Name, CurrencyCode: response.Data.Shop.CurrencyCode, Timezone: response.Data.Shop.Timezone, PlanName: response.Data.Shop.Plan.DisplayName, PrimaryDomain: response.Data.Shop.PrimaryDomain.URL}, nil
}

func (c *Client) doJSON(request *http.Request, target any) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("provider status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, target)
}
