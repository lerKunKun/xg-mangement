package shopify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) Refresh(ctx context.Context, domain, clientID, clientSecret, refreshToken string) (Token, error) {
	normalized, err := NormalizeShopDomain(domain)
	if err != nil {
		return Token{}, err
	}
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(normalized, ""), strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("create Shopify token refresh request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Token{}, &ProviderError{Code: "transport_error", Message: limitProviderMessage(err.Error()), Retryable: true}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProviderResponseBytes))
	if err != nil {
		return Token{}, &ProviderError{StatusCode: response.StatusCode, Code: "response_read_failed", Message: limitProviderMessage(err.Error()), Retryable: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		providerErr := providerHTTPError(response, body).(*ProviderError)
		if response.StatusCode == http.StatusUnauthorized && providerErr.Code == "invalid_request" && strings.Contains(strings.ToLower(providerErr.Message), "active refresh_token") {
			providerErr.Code = "reauthorization_required"
		}
		return Token{}, providerErr
	}
	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return Token{}, &ProviderError{StatusCode: response.StatusCode, Code: "invalid_response", Message: "Shopify returned invalid token JSON"}
	}
	if token.AccessToken == "" || token.RefreshToken == "" {
		return Token{}, &ProviderError{StatusCode: response.StatusCode, Code: "invalid_response", Message: "Shopify returned incomplete rotated tokens"}
	}
	return token, nil
}

func IsReauthorizationRequired(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Code == "reauthorization_required"
}
