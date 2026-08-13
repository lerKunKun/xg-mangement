package shopifysync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/xg-management/platform/backend/internal/integrations/shopify"
)

func TestTokenManagerReturnsFreshAccessTokenWithoutRefresh(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repository := &credentialRepositoryStub{connection: encryptedConnection(t, shopify.Token{AccessToken: "access-current", RefreshToken: "refresh-current"}, now.Add(time.Hour))}
	refresher := &tokenRefresherStub{}
	manager := TokenManager{Repository: repository, Cipher: plainCipher{}, Refresher: refresher, Clock: func() time.Time { return now }}

	accessToken, err := manager.AccessToken(context.Background(), "org-1", "store-1")
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if accessToken != "access-current" {
		t.Fatalf("access token = %q", accessToken)
	}
	if refresher.calls != 0 {
		t.Fatalf("refresh calls = %d, want 0", refresher.calls)
	}
}

func TestTokenManagerRotatesExpiredOfflineToken(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repository := &credentialRepositoryStub{connection: encryptedConnection(t, shopify.Token{AccessToken: "access-old", RefreshToken: "refresh-old"}, now.Add(-time.Minute))}
	refresher := &tokenRefresherStub{token: shopify.Token{AccessToken: "access-new", RefreshToken: "refresh-new", ExpiresIn: 3600, RefreshTokenExpiresIn: 7776000, Scope: "read_products"}}
	manager := TokenManager{Repository: repository, Cipher: plainCipher{}, Refresher: refresher, Clock: func() time.Time { return now }}

	accessToken, err := manager.AccessToken(context.Background(), "org-1", "store-1")
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if accessToken != "access-new" || refresher.calls != 1 {
		t.Fatalf("access token = %q, refresh calls = %d", accessToken, refresher.calls)
	}
	var saved shopify.Token
	if err := json.Unmarshal(repository.connection.EncryptedCredentials, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.RefreshToken != "refresh-new" {
		t.Fatalf("refresh token = %q, want refresh-new", saved.RefreshToken)
	}
	if repository.connection.ExpiresAt == nil || !repository.connection.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expires at = %v", repository.connection.ExpiresAt)
	}

	_, err = manager.AccessToken(context.Background(), "org-1", "store-1")
	if err != nil {
		t.Fatalf("second AccessToken() error = %v", err)
	}
	if refresher.calls != 1 {
		t.Fatalf("refresh calls = %d, want one rotation", refresher.calls)
	}
}

func TestTokenManagerMarksInvalidRefreshTokenForReauthorization(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repository := &credentialRepositoryStub{connection: encryptedConnection(t, shopify.Token{AccessToken: "access-old", RefreshToken: "refresh-old"}, now.Add(-time.Minute))}
	refresher := &tokenRefresherStub{err: &shopify.ProviderError{Code: "reauthorization_required", StatusCode: 401}}
	manager := TokenManager{Repository: repository, Cipher: plainCipher{}, Refresher: refresher, Clock: func() time.Time { return now }}

	_, err := manager.AccessToken(context.Background(), "org-1", "store-1")
	if !shopify.IsReauthorizationRequired(err) {
		t.Fatalf("AccessToken() error = %#v, want reauthorization", err)
	}
	if repository.connection.Status != "action_required" {
		t.Fatalf("connection status = %q, want action_required", repository.connection.Status)
	}
	if repository.connection.LastErrorCode != "reauthorization_required" {
		t.Fatalf("last error code = %q", repository.connection.LastErrorCode)
	}
}

func TestTokenManagerRejectsLegacyWriteScopes(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	connection := encryptedConnection(t, shopify.Token{AccessToken: "overprivileged"}, now.Add(time.Hour))
	connection.GrantedScopes = []string{"read_products", "read_themes", "write_themes"}
	repository := &credentialRepositoryStub{connection: connection}
	manager := TokenManager{Repository: repository, Cipher: plainCipher{}, Refresher: &tokenRefresherStub{}, Clock: func() time.Time { return now }}
	_, err := manager.AccessToken(context.Background(), "org-1", "store-1")
	if !shopify.IsReauthorizationRequired(err) || repository.connection.Status != "action_required" {
		t.Fatalf("AccessToken() error = %#v status = %q", err, repository.connection.Status)
	}
}

func encryptedConnection(t *testing.T, token shopify.Token, expiresAt time.Time) StoreConnection {
	t.Helper()
	payload, err := json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}
	return StoreConnection{
		OrganizationID:       "org-1",
		StoreID:              "store-1",
		AccountID:            "account-1",
		Domain:               "test.myshopify.com",
		Status:               "connected",
		EncryptedCredentials: payload,
		ExpiresAt:            &expiresAt,
		PublicConfig:         json.RawMessage(`{"client_id":"client-id","api_version":"2026-07"}`),
		EncryptedSecrets:     []byte(`{"client_secret":"client-secret"}`),
		GrantedScopes:        []string{"read_products", "read_themes"},
	}
}

type credentialRepositoryStub struct{ connection StoreConnection }

func (r *credentialRepositoryStub) WithLockedConnection(_ context.Context, organizationID, storeID string, fn func(StoreConnection) (CredentialUpdate, error)) error {
	if organizationID != r.connection.OrganizationID || storeID != r.connection.StoreID {
		return errors.New("wrong tenant connection")
	}
	update, err := fn(r.connection)
	if update.Changed {
		r.connection.EncryptedCredentials = update.EncryptedCredentials
		r.connection.ExpiresAt = update.ExpiresAt
		r.connection.RefreshExpiresAt = update.RefreshExpiresAt
		r.connection.Status = update.Status
		r.connection.LastErrorCode = update.LastErrorCode
	}
	return err
}

type plainCipher struct{}

func (plainCipher) Encrypt(value []byte) ([]byte, error) { return value, nil }
func (plainCipher) Decrypt(value []byte) ([]byte, error) { return value, nil }

type tokenRefresherStub struct {
	token shopify.Token
	err   error
	calls int
}

func (r *tokenRefresherStub) Refresh(context.Context, string, string, string, string) (shopify.Token, error) {
	r.calls++
	return r.token, r.err
}
