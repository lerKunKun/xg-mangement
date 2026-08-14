package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/admin"
	"github.com/xg-management/platform/backend/internal/auth"
	"github.com/xg-management/platform/backend/internal/security"
)

func TestShopifyInstallCreatesPendingStoreBeforeRedirect(t *testing.T) {
	cipher, err := security.NewCredentialCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := cipher.Encrypt([]byte(`{"client_secret":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := json.Marshal(shopifyPublicConfig{
		ClientID:    "client-id",
		RedirectURI: "http://localhost:3001/backend/integrations/shopify/callback",
		Scopes:      phaseOneShopifyScopes(),
		APIVersion:  shopifyAdminAPIVersion,
	})
	repository := &integrationRepositoryStub{config: admin.IntegrationConfig{
		Provider: "shopify", PublicConfig: public, EncryptedSecrets: secret, Enabled: true,
	}}
	states := auth.NewOAuthStateManager(newOAuthStateStore(), 10*time.Minute)
	handler := shopifyInstall(IntegrationDependencies{
		Repository: repository,
		Cipher:     cipher,
		States:     states,
		WebBaseURL: "http://localhost:3001",
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/integrations/shopify/install?shop=jaxdevstore.myshopify.com", nil)
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(principalContextKey, auth.Principal{OrganizationID: "org-1", UserID: "user-1"})

	handler(context)

	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.pendingDomain != "jaxdevstore.myshopify.com" {
		t.Fatalf("pending domain = %q", repository.pendingDomain)
	}
}

func TestShopifyConfigReadRepairsLegacyStoreDomainRedirect(t *testing.T) {
	public, _ := json.Marshal(shopifyPublicConfig{
		ClientID: "client-id", RedirectURI: "jaxdevstore.myshopify.com", Scopes: phaseOneShopifyScopes(), APIVersion: "unstable",
	})
	repository := &integrationRepositoryStub{config: admin.IntegrationConfig{Provider: "shopify", PublicConfig: public, Enabled: true}}
	get, _ := integrationConfigHandlers(IntegrationDependencies{Repository: repository, WebBaseURL: "http://localhost:3001"}, "shopify")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/integrations/shopify/config", nil)
	context.Set(principalContextKey, auth.Principal{OrganizationID: "org-1", UserID: "user-1"})

	get(context)

	var payload struct {
		Data struct {
			PublicConfig shopifyPublicConfig `json:"public_config"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.PublicConfig.RedirectURI != "http://localhost:3001/backend/integrations/shopify/callback" {
		t.Fatalf("redirect URI = %q", payload.Data.PublicConfig.RedirectURI)
	}
	if payload.Data.PublicConfig.APIVersion != shopifyAdminAPIVersion {
		t.Fatalf("API version = %q", payload.Data.PublicConfig.APIVersion)
	}
}

func TestDingTalkConfigReadReturnsOnlyOfficialConfigurationFields(t *testing.T) {
	public := json.RawMessage(`{"client_id":"ding-client","corp_id":"corp","agent_id":"123","approval_process_code":"PROC","redirect_uri":"https://wrong.example","scopes":["profile"],"organization_slug":"local"}`)
	repository := &integrationRepositoryStub{config: admin.IntegrationConfig{Provider: "dingtalk", PublicConfig: public, Enabled: true}}
	get, _ := integrationConfigHandlers(IntegrationDependencies{Repository: repository, WebBaseURL: "https://console.example.com"}, "dingtalk")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/integrations/dingtalk/config", nil)
	context.Set(principalContextKey, auth.Principal{OrganizationID: "org-1", UserID: "user-1"})

	get(context)

	var payload struct {
		Data struct {
			PublicConfig map[string]any `json:"public_config"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, exists := payload.Data.PublicConfig["organization_slug"]; exists {
		t.Fatal("internal organization slug leaked into DingTalk provider configuration")
	}
	if payload.Data.PublicConfig["redirect_uri"] != "https://console.example.com/backend/integrations/dingtalk/callback" {
		t.Fatalf("redirect URI = %#v", payload.Data.PublicConfig["redirect_uri"])
	}
}

func TestDingTalkConfigWriteNormalizesOfficialFields(t *testing.T) {
	cipher, err := security.NewCredentialCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	repository := &integrationRepositoryStub{}
	_, put := integrationConfigHandlers(IntegrationDependencies{Repository: repository, Cipher: cipher, WebBaseURL: "http://localhost:3001"}, "dingtalk")
	body := `{"public_config":{"client_id":" ding-client ","corp_id":" corp ","agent_id":" 123 ","approval_process_code":" PROC ","redirect_uri":"bad","scopes":["profile"],"organization_slug":"local"},"client_secret":"secret","enabled":true}`
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/v1/integrations/dingtalk/config", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(principalContextKey, auth.Principal{OrganizationID: "org-1", UserID: "user-1"})

	put(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var saved map[string]any
	if err := json.Unmarshal(repository.savedPublic, &saved); err != nil {
		t.Fatal(err)
	}
	if _, exists := saved["organization_slug"]; exists {
		t.Fatal("internal organization slug was saved as DingTalk configuration")
	}
	if saved["redirect_uri"] != "http://localhost:3001/backend/integrations/dingtalk/callback" {
		t.Fatalf("saved redirect URI = %#v", saved["redirect_uri"])
	}
}

func TestEnabledIntegrationRequiresAConfiguredSecret(t *testing.T) {
	cipher, err := security.NewCredentialCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	repository := &integrationRepositoryStub{config: admin.IntegrationConfig{Provider: "shopify", PublicConfig: json.RawMessage(`{}`)}}
	_, put := integrationConfigHandlers(IntegrationDependencies{Repository: repository, Cipher: cipher, WebBaseURL: "http://localhost:3001"}, "shopify")
	body := `{"public_config":{"client_id":"client-id","redirect_uri":"jaxdevstore.myshopify.com"},"client_secret":"","enabled":true}`
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/v1/integrations/shopify/config", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(principalContextKey, auth.Principal{OrganizationID: "org-1", UserID: "user-1"})

	put(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if repository.upsertCalled {
		t.Fatal("invalid enabled integration was persisted")
	}
}

func TestLoadShopifyConfigUsesCanonicalCallback(t *testing.T) {
	cipher, err := security.NewCredentialCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := cipher.Encrypt([]byte(`{"client_secret":"secret"}`))
	public, _ := json.Marshal(shopifyPublicConfig{ClientID: "client-id", RedirectURI: "jaxdevstore.myshopify.com", Scopes: phaseOneShopifyScopes()})
	repository := &integrationRepositoryStub{config: admin.IntegrationConfig{Provider: "shopify", PublicConfig: public, EncryptedSecrets: secret, Enabled: true}}

	config, _, err := loadShopifyConfig(context.Background(), IntegrationDependencies{Repository: repository, Cipher: cipher, WebBaseURL: "http://localhost:3001"}, "org-1")
	if err != nil {
		t.Fatal(err)
	}
	if config.RedirectURI != "http://localhost:3001/backend/integrations/shopify/callback" {
		t.Fatalf("redirect URI = %q", config.RedirectURI)
	}
}

type integrationRepositoryStub struct {
	admin.Repository
	config        admin.IntegrationConfig
	pendingDomain string
	savedPublic   json.RawMessage
	upsertCalled  bool
}

func (r *integrationRepositoryStub) GetIntegrationConfig(context.Context, string, string) (admin.IntegrationConfig, error) {
	return r.config, nil
}

func (r *integrationRepositoryStub) EnsurePendingShopifyStore(_ context.Context, _ string, domain string) (admin.StoreDetails, error) {
	r.pendingDomain = domain
	return admin.StoreDetails{ID: "store-1", Domain: domain, Status: "pending"}, nil
}

func (r *integrationRepositoryStub) UpsertIntegrationConfig(_ context.Context, _ string, provider string, public json.RawMessage, secrets []byte, enabled bool, _ string) (admin.IntegrationConfig, error) {
	r.savedPublic = append(json.RawMessage(nil), public...)
	r.upsertCalled = true
	return admin.IntegrationConfig{Provider: provider, PublicConfig: public, EncryptedSecrets: secrets, Enabled: enabled, UpdatedAt: time.Now()}, nil
}

type oauthStateStore struct {
	values map[string][]byte
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{values: map[string][]byte{}}
}

func (s *oauthStateStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.values[key] = append([]byte(nil), value...)
	return nil
}

func (s *oauthStateStore) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return append([]byte(nil), value...), nil
}

func (s *oauthStateStore) GetDelete(ctx context.Context, key string) ([]byte, error) {
	value, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	delete(s.values, key)
	return value, nil
}

func (s *oauthStateStore) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}
