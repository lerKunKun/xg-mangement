package dingtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	authorizeEndpoint = "https://login.dingtalk.com/oauth2/auth"
	tokenEndpoint     = "https://api.dingtalk.com/v1.0/oauth2/userAccessToken"
	profileEndpoint   = "https://api.dingtalk.com/v1.0/contact/users/me"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
}

type Token struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpireIn     int64  `json:"expireIn"`
	CorpID       string `json:"corpId"`
}

type Profile struct {
	Nick      string `json:"nick"`
	UnionID   string `json:"unionId"`
	OpenID    string `json:"openId"`
	AvatarURL string `json:"avatarUrl"`
	Mobile    string `json:"mobile"`
	Email     string `json:"email"`
	StateCode string `json:"stateCode"`
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client { return &Client{httpClient: &http.Client{Timeout: 15 * time.Second}} }

func AuthorizationURL(cfg Config, state string) (string, error) {
	if cfg.ClientID == "" || cfg.RedirectURI == "" || state == "" {
		return "", fmt.Errorf("DingTalk OAuth configuration is incomplete")
	}
	endpoint, _ := url.Parse(authorizeEndpoint)
	query := endpoint.Query()
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", cfg.RedirectURI)
	query.Set("state", state)
	query.Set("response_type", "code")
	query.Set("prompt", "consent")
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "corpid"}
	}
	query.Set("scope", strings.Join(scopes, " "))
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func (c *Client) Exchange(ctx context.Context, cfg Config, code string) (Token, error) {
	payload, _ := json.Marshal(map[string]string{"clientId": cfg.ClientID, "clientSecret": cfg.ClientSecret, "code": code, "grantType": "authorization_code"})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, bytes.NewReader(payload))
	if err != nil {
		return Token{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	var token Token
	if err := c.doJSON(request, &token); err != nil {
		return Token{}, fmt.Errorf("exchange DingTalk authorization code: %w", err)
	}
	if token.AccessToken == "" {
		return Token{}, fmt.Errorf("DingTalk returned an empty access token")
	}
	return token, nil
}

func (c *Client) Profile(ctx context.Context, accessToken string) (Profile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, profileEndpoint, nil)
	if err != nil {
		return Profile{}, err
	}
	request.Header.Set("x-acs-dingtalk-access-token", accessToken)
	var profile Profile
	if err := c.doJSON(request, &profile); err != nil {
		return Profile{}, fmt.Errorf("load DingTalk profile: %w", err)
	}
	if profile.UnionID == "" && profile.OpenID == "" {
		return Profile{}, fmt.Errorf("DingTalk profile has no stable identity")
	}
	return profile, nil
}

func (c *Client) doJSON(request *http.Request, target any) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("provider status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode provider response: %w", err)
	}
	return nil
}
