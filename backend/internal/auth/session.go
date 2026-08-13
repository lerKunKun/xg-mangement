package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const SessionCookieName = "xg_session"

type KeyValueStore interface {
	Set(context.Context, string, []byte, time.Duration) error
	Get(context.Context, string) ([]byte, error)
	GetDelete(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type SessionRecord struct {
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
}

type SessionManager struct {
	store KeyValueStore
	ttl   time.Duration
}

func NewSessionManager(store KeyValueStore, ttl time.Duration) *SessionManager {
	return &SessionManager{store: store, ttl: ttl}
}

func (m *SessionManager) Create(ctx context.Context, userID, organizationID string) (string, error) {
	if m == nil || m.store == nil || strings.TrimSpace(userID) == "" || strings.TrimSpace(organizationID) == "" {
		return "", fmt.Errorf("invalid session")
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(SessionRecord{UserID: userID, OrganizationID: organizationID})
	if err != nil {
		return "", fmt.Errorf("encode session: %w", err)
	}
	if err := m.store.Set(ctx, "session:"+token, payload, m.ttl); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}
	return token, nil
}

func (m *SessionManager) Read(ctx context.Context, token string) (SessionRecord, error) {
	if m == nil || m.store == nil || strings.TrimSpace(token) == "" {
		return SessionRecord{}, fmt.Errorf("session not found")
	}
	payload, err := m.store.Get(ctx, "session:"+token)
	if err != nil {
		return SessionRecord{}, fmt.Errorf("read session: %w", err)
	}
	var session SessionRecord
	if err := json.Unmarshal(payload, &session); err != nil || session.UserID == "" || session.OrganizationID == "" {
		return SessionRecord{}, fmt.Errorf("invalid session payload")
	}
	return session, nil
}

func (m *SessionManager) Delete(ctx context.Context, token string) error {
	if m == nil || m.store == nil || token == "" {
		return nil
	}
	return m.store.Delete(ctx, "session:"+token)
}

type PrincipalResolver interface {
	ResolvePrincipal(context.Context, string, string) (Principal, error)
}

type SessionAuthenticator struct {
	Sessions   *SessionManager
	Principals PrincipalResolver
}

func (a SessionAuthenticator) Authenticate(request *http.Request) (Principal, bool) {
	if a.Sessions == nil || a.Principals == nil {
		return Principal{}, false
	}
	cookie, err := request.Cookie(SessionCookieName)
	if err != nil {
		return Principal{}, false
	}
	session, err := a.Sessions.Read(request.Context(), cookie.Value)
	if err != nil {
		return Principal{}, false
	}
	principal, err := a.Principals.ResolvePrincipal(request.Context(), session.UserID, session.OrganizationID)
	if err != nil || principal.UserID != session.UserID || principal.OrganizationID != session.OrganizationID {
		return Principal{}, false
	}
	return principal, true
}

func NewSessionCookie(token string, secure bool, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	}
}

func ExpiredSessionCookie(secure bool) *http.Cookie {
	cookie := NewSessionCookie("", secure, -time.Hour)
	cookie.MaxAge = -1
	return cookie
}

type OAuthState struct {
	Provider       string `json:"provider"`
	OrganizationID string `json:"organization_id"`
	UserID         string `json:"user_id,omitempty"`
	Subject        string `json:"subject,omitempty"`
	ReturnTo       string `json:"return_to,omitempty"`
}

type OAuthStateManager struct {
	store KeyValueStore
	ttl   time.Duration
}

func NewOAuthStateManager(store KeyValueStore, ttl time.Duration) *OAuthStateManager {
	return &OAuthStateManager{store: store, ttl: ttl}
}

func (m *OAuthStateManager) Create(ctx context.Context, state OAuthState) (string, error) {
	if m == nil || m.store == nil || state.Provider == "" || state.OrganizationID == "" {
		return "", fmt.Errorf("invalid OAuth state")
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode OAuth state: %w", err)
	}
	if err := m.store.Set(ctx, "oauth_state:"+token, payload, m.ttl); err != nil {
		return "", fmt.Errorf("store OAuth state: %w", err)
	}
	return token, nil
}

func (m *OAuthStateManager) Consume(ctx context.Context, token, provider string) (OAuthState, error) {
	if m == nil || m.store == nil || token == "" {
		return OAuthState{}, fmt.Errorf("OAuth state not found")
	}
	payload, err := m.store.GetDelete(ctx, "oauth_state:"+token)
	if err != nil {
		return OAuthState{}, fmt.Errorf("consume OAuth state: %w", err)
	}
	var state OAuthState
	if err := json.Unmarshal(payload, &state); err != nil {
		return OAuthState{}, fmt.Errorf("decode OAuth state: %w", err)
	}
	if state.Provider != provider || state.OrganizationID == "" {
		return OAuthState{}, fmt.Errorf("OAuth state provider mismatch")
	}
	return state, nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
