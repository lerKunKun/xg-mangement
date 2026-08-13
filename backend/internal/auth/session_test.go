package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionAuthenticatorResolvesCurrentPrincipal(t *testing.T) {
	store := newMemoryStore()
	manager := NewSessionManager(store, time.Hour)
	token, err := manager.Create(context.Background(), "user-1", "org-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	resolver := resolverStub{principal: Principal{
		UserID: "user-1", OrganizationID: "org-1", DisplayName: "Owner", Permissions: []string{"rbac:manage"},
	}}
	authenticator := SessionAuthenticator{Sessions: manager, Principals: resolver}
	request := httptest.NewRequest("GET", "/api/v1/me", nil)
	request.AddCookie(NewSessionCookie(token, false, time.Hour))

	principal, ok := authenticator.Authenticate(request)
	if !ok {
		t.Fatal("session was not authenticated")
	}
	if principal.DisplayName != "Owner" || len(principal.Permissions) != 1 {
		t.Fatalf("principal = %#v", principal)
	}
}

func TestOAuthStateIsConsumedOnlyOnce(t *testing.T) {
	store := newMemoryStore()
	manager := NewOAuthStateManager(store, 10*time.Minute)
	state := OAuthState{Provider: "shopify", OrganizationID: "org-1", UserID: "user-1", Subject: "demo.myshopify.com"}
	token, err := manager.Create(context.Background(), state)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("empty state token")
	}

	got, err := manager.Consume(context.Background(), token, "shopify")
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if got.OrganizationID != "org-1" || got.Subject != "demo.myshopify.com" {
		t.Fatalf("state = %#v", got)
	}
	if _, err := manager.Consume(context.Background(), token, "shopify"); err == nil {
		t.Fatal("state was consumed twice")
	}
}

func TestOAuthStateRejectsWrongProvider(t *testing.T) {
	manager := NewOAuthStateManager(newMemoryStore(), 10*time.Minute)
	token, err := manager.Create(context.Background(), OAuthState{Provider: "dingtalk", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := manager.Consume(context.Background(), token, "shopify"); err == nil {
		t.Fatal("wrong provider accepted")
	}
}

type resolverStub struct{ principal Principal }

func (r resolverStub) ResolvePrincipal(context.Context, string, string) (Principal, error) {
	return r.principal, nil
}

type memoryStore struct{ values map[string][]byte }

func newMemoryStore() *memoryStore { return &memoryStore{values: map[string][]byte{}} }

func (s *memoryStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.values[key] = append([]byte(nil), value...)
	return nil
}

func (s *memoryStore) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), value...), nil
}

func (s *memoryStore) GetDelete(_ context.Context, key string) ([]byte, error) {
	value, err := s.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	delete(s.values, key)
	return value, nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}
