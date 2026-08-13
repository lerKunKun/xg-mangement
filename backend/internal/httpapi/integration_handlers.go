package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/admin"
	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/integrations/dingtalk"
	"github.com/xg-management/platform/backend/internal/integrations/shopify"
	"github.com/xg-management/platform/backend/internal/jobs"
	"github.com/xg-management/platform/backend/internal/security"
)

type OAuthStateService interface {
	Create(context.Context, auth.OAuthState) (string, error)
	Consume(context.Context, string, string) (auth.OAuthState, error)
}
type JobPublisher interface {
	Publish(context.Context, jobs.Envelope) error
}
type DingTalkProvider interface {
	Exchange(context.Context, dingtalk.Config, string) (dingtalk.Token, error)
	Profile(context.Context, string) (dingtalk.Profile, error)
}
type ShopifyProvider interface {
	Exchange(context.Context, string, shopify.OAuthConfig, string) (shopify.Token, error)
	Shop(context.Context, string, string, string) (shopify.Shop, error)
}

type IntegrationDependencies struct {
	Repository     admin.Repository
	Cipher         *security.CredentialCipher
	States         *auth.OAuthStateManager
	Sessions       *auth.SessionManager
	DingTalk       *dingtalk.Client
	Shopify        *shopify.Client
	Jobs           JobPublisher
	WebBaseURL     string
	SecureCookies  bool
	SessionTTL     time.Duration
	PolicyReloader PolicyReloader
}

type dingTalkPublicConfig struct {
	ClientID         string   `json:"client_id"`
	RedirectURI      string   `json:"redirect_uri"`
	Scopes           []string `json:"scopes"`
	OrganizationSlug string   `json:"organization_slug"`
	CorpID           string   `json:"corp_id"`
}
type dingTalkSecrets struct {
	ClientSecret string `json:"client_secret"`
}
type shopifyPublicConfig struct {
	ClientID    string   `json:"client_id"`
	RedirectURI string   `json:"redirect_uri"`
	Scopes      []string `json:"scopes"`
	APIVersion  string   `json:"api_version"`
}
type shopifySecrets struct {
	ClientSecret string `json:"client_secret"`
}

func integrationConfigHandlers(deps IntegrationDependencies, provider string) (gin.HandlerFunc, gin.HandlerFunc) {
	get := func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		item, err := deps.Repository.GetIntegrationConfig(c, principal.OrganizationID, provider)
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, gin.H{"provider": provider, "public_config": item.PublicConfig, "enabled": item.Enabled, "secret_configured": len(item.EncryptedSecrets) > 0, "updated_at": item.UpdatedAt})
	}
	put := func(c *gin.Context) {
		var input struct {
			PublicConfig json.RawMessage `json:"public_config"`
			ClientSecret string          `json:"client_secret"`
			Enabled      bool            `json:"enabled"`
		}
		if !bindJSON(c, &input) || !json.Valid(input.PublicConfig) {
			invalidInput(c, "public_config must be valid JSON")
			return
		}
		principal, _ := currentPrincipal(c)
		var encrypted []byte
		if input.ClientSecret != "" {
			payload, _ := json.Marshal(map[string]string{"client_secret": input.ClientSecret})
			var err error
			encrypted, err = deps.Cipher.Encrypt(payload)
			if err != nil {
				internalError(c)
				return
			}
		}
		item, err := deps.Repository.UpsertIntegrationConfig(c, principal.OrganizationID, provider, input.PublicConfig, encrypted, input.Enabled, principal.UserID)
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, gin.H{"provider": provider, "public_config": item.PublicConfig, "enabled": item.Enabled, "secret_configured": len(item.EncryptedSecrets) > 0, "updated_at": item.UpdatedAt})
	}
	return get, put
}

func dingTalkLogin(deps IntegrationDependencies, authenticated bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var organizationID, userID string
		if authenticated {
			principal, _ := currentPrincipal(c)
			organizationID = principal.OrganizationID
			userID = principal.UserID
		} else {
			slug := c.Query("organization")
			var err error
			organizationID, err = deps.Repository.OrganizationIDBySlug(c, slug)
			if err != nil {
				respondError(c, http.StatusNotFound, "organization_not_found", "The organization login was not found.")
				return
			}
		}
		cfg, secret, err := loadDingTalkConfig(c, deps, organizationID)
		if err != nil {
			integrationNotConfigured(c)
			return
		}
		state, err := deps.States.Create(c, auth.OAuthState{Provider: "dingtalk", OrganizationID: organizationID, UserID: userID, ReturnTo: safeReturnTo(c.Query("return_to"), "/dashboard")})
		if err != nil {
			internalError(c)
			return
		}
		target, err := dingtalk.AuthorizationURL(dingtalk.Config{ClientID: cfg.ClientID, ClientSecret: secret.ClientSecret, RedirectURI: cfg.RedirectURI, Scopes: cfg.Scopes}, state)
		if err != nil {
			integrationNotConfigured(c)
			return
		}
		c.Redirect(http.StatusFound, target)
	}
}

func dingTalkCallback(deps IntegrationDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		state, err := deps.States.Consume(c, c.Query("state"), "dingtalk")
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_oauth_state", "The DingTalk login state is invalid or expired.")
			return
		}
		code := c.Query("authCode")
		if code == "" {
			code = c.Query("code")
		}
		if code == "" {
			invalidInput(c, "DingTalk authorization code is missing")
			return
		}
		cfg, secret, err := loadDingTalkConfig(c, deps, state.OrganizationID)
		if err != nil {
			integrationNotConfigured(c)
			return
		}
		token, err := deps.DingTalk.Exchange(c, dingtalk.Config{ClientID: cfg.ClientID, ClientSecret: secret.ClientSecret, RedirectURI: cfg.RedirectURI, Scopes: cfg.Scopes}, code)
		if err != nil {
			providerFailure(c, "dingtalk_oauth_failed")
			return
		}
		if cfg.CorpID == "" || token.CorpID != cfg.CorpID {
			respondError(c, http.StatusForbidden, "dingtalk_organization_mismatch", "The DingTalk user did not authorize the configured organization.")
			return
		}
		profile, err := deps.DingTalk.Profile(c, token.AccessToken)
		if err != nil {
			providerFailure(c, "dingtalk_profile_failed")
			return
		}
		identityID := profile.UnionID
		if identityID == "" {
			identityID = profile.OpenID
		}
		metadata, _ := json.Marshal(profile)
		userID, err := deps.Repository.UpsertDingTalkIdentity(c, admin.DingTalkIdentityInput{OrganizationID: state.OrganizationID, UserID: state.UserID, ProviderUserID: identityID, DisplayName: profile.Nick, Email: profile.Email, Metadata: metadata})
		if err != nil {
			internalError(c)
			return
		}
		if !reloadPolicy(c, deps.PolicyReloader) {
			return
		}
		sessionToken, err := deps.Sessions.Create(c, userID, state.OrganizationID)
		if err != nil {
			internalError(c)
			return
		}
		http.SetCookie(c.Writer, auth.NewSessionCookie(sessionToken, deps.SecureCookies, deps.SessionTTL))
		c.Redirect(http.StatusFound, strings.TrimRight(deps.WebBaseURL, "/")+safeReturnTo(state.ReturnTo, "/dashboard"))
	}
}

func listDingTalkUsers(deps IntegrationDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		items, err := deps.Repository.ListDingTalkUsers(c, principal.OrganizationID)
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, items)
	}
}

func shopifyInstall(deps IntegrationDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		domain, err := shopify.NormalizeShopDomain(c.Query("shop"))
		if err != nil {
			invalidInput(c, "shop must be a valid *.myshopify.com domain")
			return
		}
		cfg, secret, err := loadShopifyConfig(c, deps, principal.OrganizationID)
		if err != nil {
			integrationNotConfigured(c)
			return
		}
		state, err := deps.States.Create(c, auth.OAuthState{Provider: "shopify", OrganizationID: principal.OrganizationID, UserID: principal.UserID, Subject: domain, ReturnTo: "/stores"})
		if err != nil {
			internalError(c)
			return
		}
		target, err := shopify.AuthorizationURL(domain, shopify.OAuthConfig{ClientID: cfg.ClientID, ClientSecret: secret.ClientSecret, RedirectURI: cfg.RedirectURI, Scopes: cfg.Scopes, APIVersion: cfg.APIVersion}, state)
		if err != nil {
			invalidInput(c, err.Error())
			return
		}
		cookie := &http.Cookie{Name: "xg_shopify_state", Value: state, Path: "/", HttpOnly: true, Secure: deps.SecureCookies, SameSite: http.SameSiteLaxMode, MaxAge: 600}
		http.SetCookie(c.Writer, cookie)
		c.Redirect(http.StatusFound, target)
	}
}

func shopifyCallback(deps IntegrationDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Request.URL.Query()
		stateValue := query.Get("state")
		cookie, err := c.Request.Cookie("xg_shopify_state")
		if err != nil || cookie.Value != stateValue {
			respondError(c, http.StatusBadRequest, "invalid_oauth_state", "The Shopify authorization state is invalid.")
			return
		}
		state, err := deps.States.Consume(c, stateValue, "shopify")
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_oauth_state", "The Shopify authorization state is invalid or expired.")
			return
		}
		domain, err := shopify.NormalizeShopDomain(query.Get("shop"))
		if err != nil || domain != state.Subject {
			invalidInput(c, "Shopify shop domain mismatch")
			return
		}
		cfg, secret, err := loadShopifyConfig(c, deps, state.OrganizationID)
		if err != nil {
			integrationNotConfigured(c)
			return
		}
		if !shopify.VerifyCallbackHMAC(query, secret.ClientSecret) {
			respondError(c, http.StatusBadRequest, "invalid_shopify_hmac", "The Shopify callback signature is invalid.")
			return
		}
		token, err := deps.Shopify.Exchange(c, domain, shopify.OAuthConfig{ClientID: cfg.ClientID, ClientSecret: secret.ClientSecret, RedirectURI: cfg.RedirectURI, Scopes: cfg.Scopes, APIVersion: cfg.APIVersion}, query.Get("code"))
		if err != nil {
			providerFailure(c, "shopify_oauth_failed")
			return
		}
		if !scopesContain(token.Scope, cfg.Scopes) {
			respondError(c, http.StatusBadRequest, "shopify_scope_mismatch", "Shopify did not grant every required access scope.")
			return
		}
		shop, err := deps.Shopify.Shop(c, domain, token.AccessToken, cfg.APIVersion)
		if err != nil {
			providerFailure(c, "shopify_shop_failed")
			return
		}
		tokens, _ := json.Marshal(token)
		encrypted, err := deps.Cipher.Encrypt(tokens)
		if err != nil {
			internalError(c)
			return
		}
		now := time.Now()
		var expiresAt, refreshExpiresAt *time.Time
		if token.ExpiresIn > 0 {
			value := now.Add(time.Duration(token.ExpiresIn) * time.Second)
			expiresAt = &value
		}
		if token.RefreshTokenExpiresIn > 0 {
			value := now.Add(time.Duration(token.RefreshTokenExpiresIn) * time.Second)
			refreshExpiresAt = &value
		}
		_, err = deps.Repository.UpsertShopifyAuthorization(c, admin.ShopifyAuthorization{OrganizationID: state.OrganizationID, Domain: domain, DisplayName: shop.Name, ShopifyGID: shop.ID, PrimaryDomain: shop.PrimaryDomain, Currency: shop.CurrencyCode, Timezone: shop.Timezone, PlanName: shop.PlanName, Scopes: strings.Split(token.Scope, ","), EncryptedTokens: encrypted, ExpiresAt: expiresAt, RefreshExpiresAt: refreshExpiresAt})
		if err != nil {
			internalError(c)
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{Name: "xg_shopify_state", Path: "/", MaxAge: -1, HttpOnly: true, Secure: deps.SecureCookies, SameSite: http.SameSiteLaxMode})
		c.Redirect(http.StatusFound, strings.TrimRight(deps.WebBaseURL, "/")+"/stores?connected=1")
	}
}

func getStore(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		item, err := repository.GetStore(c, principal.OrganizationID, c.Param("id"))
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, item)
	}
}
func updateStore(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		if !bindJSON(c, &input) || strings.TrimSpace(input.Name) == "" || (input.Status != "connected" && input.Status != "action_required" && input.Status != "disconnected") {
			invalidInput(c, "name and valid status are required")
			return
		}
		principal, _ := currentPrincipal(c)
		item, err := repository.UpdateStore(c, principal.OrganizationID, c.Param("id"), strings.TrimSpace(input.Name), input.Status)
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, item)
	}
}
func disconnectStore(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		if err := repository.DisconnectStore(c, principal.OrganizationID, c.Param("id")); err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, gin.H{"disconnected": true})
	}
}

func syncStore(deps IntegrationDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.Jobs == nil {
			respondError(c, http.StatusServiceUnavailable, "queue_unavailable", "The background job queue is unavailable.")
			return
		}
		principal, _ := currentPrincipal(c)
		store, err := deps.Repository.GetStore(c, principal.OrganizationID, c.Param("id"))
		if err != nil || store.Status != "connected" {
			respondError(c, http.StatusBadRequest, "store_not_connected", "Only a connected store can be synchronized.")
			return
		}
		payload, _ := json.Marshal(gin.H{"store_id": store.ID, "shop_domain": store.Domain})
		envelope := jobs.Envelope{Version: 1, ID: newRequestID(), Type: jobs.TypeShopifyStoreSyncRequested, OrganizationID: principal.OrganizationID, OccurredAt: time.Now().UTC(), Payload: payload}
		if err := deps.Jobs.Publish(c, envelope); err != nil {
			respondError(c, http.StatusServiceUnavailable, "queue_publish_failed", "The synchronization job could not be queued.")
			return
		}
		respondData(c, http.StatusAccepted, gin.H{"job_id": envelope.ID, "status": "queued"})
	}
}

func loadDingTalkConfig(c context.Context, deps IntegrationDependencies, organizationID string) (dingTalkPublicConfig, dingTalkSecrets, error) {
	item, err := deps.Repository.GetIntegrationConfig(c, organizationID, "dingtalk")
	if err != nil || !item.Enabled || len(item.EncryptedSecrets) == 0 {
		return dingTalkPublicConfig{}, dingTalkSecrets{}, errors.New("not configured")
	}
	var public dingTalkPublicConfig
	var secret dingTalkSecrets
	if json.Unmarshal(item.PublicConfig, &public) != nil {
		return public, secret, errors.New("invalid config")
	}
	plain, err := deps.Cipher.Decrypt(item.EncryptedSecrets)
	if err != nil || json.Unmarshal(plain, &secret) != nil || secret.ClientSecret == "" {
		return public, secret, errors.New("invalid secret")
	}
	return public, secret, nil
}
func loadShopifyConfig(c context.Context, deps IntegrationDependencies, organizationID string) (shopifyPublicConfig, shopifySecrets, error) {
	item, err := deps.Repository.GetIntegrationConfig(c, organizationID, "shopify")
	if err != nil || !item.Enabled || len(item.EncryptedSecrets) == 0 {
		return shopifyPublicConfig{}, shopifySecrets{}, errors.New("not configured")
	}
	var public shopifyPublicConfig
	var secret shopifySecrets
	if json.Unmarshal(item.PublicConfig, &public) != nil {
		return public, secret, errors.New("invalid config")
	}
	plain, err := deps.Cipher.Decrypt(item.EncryptedSecrets)
	if err != nil || json.Unmarshal(plain, &secret) != nil || secret.ClientSecret == "" {
		return public, secret, errors.New("invalid secret")
	}
	return public, secret, nil
}
func scopesContain(granted string, required []string) bool {
	values := map[string]bool{}
	for _, scope := range strings.Split(granted, ",") {
		values[strings.TrimSpace(scope)] = true
	}
	for _, scope := range required {
		if !values[scope] {
			return false
		}
	}
	return true
}
func integrationNotConfigured(c *gin.Context) {
	respondError(c, http.StatusServiceUnavailable, "integration_not_configured", "The organization integration is not configured and enabled.")
}
func providerFailure(c *gin.Context, code string) {
	respondError(c, http.StatusBadGateway, code, "The external provider request failed.")
}

var _ = url.Values{}
