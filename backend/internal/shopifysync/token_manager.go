package shopifysync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xg-management/platform/backend/internal/integrations/shopify"
)

const tokenRefreshSkew = 5 * time.Minute

var phaseOneTokenScopes = []string{"read_products", "read_themes"}

type CredentialCipher interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

type CredentialRepository interface {
	WithLockedConnection(context.Context, string, string, func(StoreConnection) (CredentialUpdate, error)) error
}

type TokenRefresher interface {
	Refresh(context.Context, string, string, string, string) (shopify.Token, error)
}

type TokenManager struct {
	Repository CredentialRepository
	Cipher     CredentialCipher
	Refresher  TokenRefresher
	Clock      func() time.Time
}

func (m TokenManager) AccessToken(ctx context.Context, organizationID, storeID string) (string, error) {
	if m.Repository == nil || m.Cipher == nil || m.Refresher == nil {
		return "", fmt.Errorf("Shopify token manager is not configured")
	}
	now := time.Now().UTC()
	if m.Clock != nil {
		now = m.Clock().UTC()
	}
	var accessToken string
	err := m.Repository.WithLockedConnection(ctx, organizationID, storeID, func(connection StoreConnection) (CredentialUpdate, error) {
		if !exactScopeSet(connection.GrantedScopes, phaseOneTokenScopes) {
			err := &shopify.ProviderError{StatusCode: 401, Code: "reauthorization_required", Message: "Shopify authorization must be renewed with the Phase 1 read-only scopes"}
			return actionRequiredUpdate(), err
		}
		token, err := m.decryptToken(connection.EncryptedCredentials)
		if err != nil {
			return CredentialUpdate{}, err
		}
		if connection.ExpiresAt == nil || now.Add(tokenRefreshSkew).Before(*connection.ExpiresAt) {
			if strings.TrimSpace(token.AccessToken) == "" {
				return CredentialUpdate{}, fmt.Errorf("Shopify credentials do not contain an access token")
			}
			accessToken = token.AccessToken
			return CredentialUpdate{}, nil
		}

		if strings.TrimSpace(token.RefreshToken) == "" {
			err := &shopify.ProviderError{StatusCode: 401, Code: "reauthorization_required", Message: "Shopify offline session must be authorized again"}
			return actionRequiredUpdate(), err
		}
		clientID, clientSecret, err := m.clientCredentials(connection)
		if err != nil {
			return CredentialUpdate{}, err
		}
		rotated, err := m.Refresher.Refresh(ctx, connection.Domain, clientID, clientSecret, token.RefreshToken)
		if err != nil {
			if shopify.IsReauthorizationRequired(err) {
				return actionRequiredUpdate(), err
			}
			return CredentialUpdate{}, err
		}
		payload, err := json.Marshal(rotated)
		if err != nil {
			return CredentialUpdate{}, fmt.Errorf("encode rotated Shopify credentials: %w", err)
		}
		encrypted, err := m.Cipher.Encrypt(payload)
		if err != nil {
			return CredentialUpdate{}, fmt.Errorf("encrypt rotated Shopify credentials: %w", err)
		}
		expiresAt := expiryFromDuration(now, rotated.ExpiresIn)
		refreshExpiresAt := expiryFromDuration(now, rotated.RefreshTokenExpiresIn)
		accessToken = rotated.AccessToken
		return CredentialUpdate{
			Changed:              true,
			EncryptedCredentials: encrypted,
			ExpiresAt:            expiresAt,
			RefreshExpiresAt:     refreshExpiresAt,
			Status:               "connected",
		}, nil
	})
	if err != nil {
		return "", err
	}
	return accessToken, nil
}

func exactScopeSet(granted, required []string) bool {
	if len(granted) != len(required) {
		return false
	}
	values := make(map[string]bool, len(granted))
	for _, scope := range granted {
		values[strings.TrimSpace(scope)] = true
	}
	for _, scope := range required {
		if !values[scope] {
			return false
		}
	}
	return true
}

func (m TokenManager) decryptToken(ciphertext []byte) (shopify.Token, error) {
	plaintext, err := m.Cipher.Decrypt(ciphertext)
	if err != nil {
		return shopify.Token{}, fmt.Errorf("decrypt Shopify credentials: %w", err)
	}
	var token shopify.Token
	if err := json.Unmarshal(plaintext, &token); err != nil {
		return shopify.Token{}, fmt.Errorf("decode Shopify credentials: %w", err)
	}
	return token, nil
}

func (m TokenManager) clientCredentials(connection StoreConnection) (string, string, error) {
	var public struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(connection.PublicConfig, &public); err != nil {
		return "", "", fmt.Errorf("decode Shopify public configuration: %w", err)
	}
	secretJSON, err := m.Cipher.Decrypt(connection.EncryptedSecrets)
	if err != nil {
		return "", "", fmt.Errorf("decrypt Shopify secret configuration: %w", err)
	}
	var secrets struct {
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(secretJSON, &secrets); err != nil {
		return "", "", fmt.Errorf("decode Shopify secret configuration: %w", err)
	}
	if strings.TrimSpace(public.ClientID) == "" || strings.TrimSpace(secrets.ClientSecret) == "" {
		return "", "", fmt.Errorf("Shopify client credentials are incomplete")
	}
	return public.ClientID, secrets.ClientSecret, nil
}

func actionRequiredUpdate() CredentialUpdate {
	return CredentialUpdate{Changed: true, Status: "action_required", LastErrorCode: "reauthorization_required"}
}

func expiryFromDuration(now time.Time, seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := now.Add(time.Duration(seconds) * time.Second)
	return &value
}
