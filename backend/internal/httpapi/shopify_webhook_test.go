package httpapi_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xg-management/platform/backend/internal/httpapi"
)

func TestShopifyWebhookValidatesHeadersSignatureAndDuplicates(t *testing.T) {
	body := []byte(`{"id":123}`)
	repository := &webhookRepositoryStub{target: httpapi.ShopifyWebhookTarget{OrganizationID: "org-1", StoreID: "store-1", EncryptedSecrets: []byte(`{"client_secret":"webhook-secret"}`)}}
	router := httpapi.NewRouter(httpapi.Dependencies{Webhooks: &httpapi.ShopifyWebhookDependencies{Repository: repository, Cipher: webhookCipherStub{}}})

	t.Run("missing headers", func(t *testing.T) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/shopify", strings.NewReader(string(body))))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", response.Code)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		request := webhookRequest(body, "invalid")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})

	t.Run("accepted", func(t *testing.T) {
		request := webhookRequest(body, webhookSignature(body, "webhook-secret"))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusAccepted || repository.events != 1 {
			t.Fatalf("status = %d, events = %d, body = %s", response.Code, repository.events, response.Body.String())
		}
		if repository.event.OrganizationID != "org-1" || repository.event.StoreID != "store-1" || repository.event.Topic != "products/update" {
			t.Fatalf("event = %#v", repository.event)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		repository.duplicate = true
		request := webhookRequest(body, webhookSignature(body, "webhook-secret"))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
	})
}

func TestShopifyWebhookReturnsNotFoundForUnknownStore(t *testing.T) {
	repository := &webhookRepositoryStub{resolveErr: httpapi.ErrShopifyWebhookStoreNotFound}
	router := httpapi.NewRouter(httpapi.Dependencies{Webhooks: &httpapi.ShopifyWebhookDependencies{Repository: repository, Cipher: webhookCipherStub{}}})
	request := webhookRequest([]byte(`{}`), "unused")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func webhookRequest(body []byte, signature string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/shopify", strings.NewReader(string(body)))
	request.Header.Set("X-Shopify-Hmac-Sha256", signature)
	request.Header.Set("X-Shopify-Shop-Domain", "test.myshopify.com")
	request.Header.Set("X-Shopify-Topic", "products/update")
	request.Header.Set("X-Shopify-Webhook-Id", "webhook-1")
	return request
}

func webhookSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type webhookCipherStub struct{}

func (webhookCipherStub) Decrypt(value []byte) ([]byte, error) { return value, nil }

type webhookRepositoryStub struct {
	target     httpapi.ShopifyWebhookTarget
	resolveErr error
	duplicate  bool
	events     int
	event      httpapi.ShopifyWebhookEvent
}

func (r *webhookRepositoryStub) ResolveShopifyWebhookTarget(context.Context, string) (httpapi.ShopifyWebhookTarget, error) {
	return r.target, r.resolveErr
}

func (r *webhookRepositoryStub) RecordShopifyEventAndOutbox(_ context.Context, event httpapi.ShopifyWebhookEvent) (bool, error) {
	r.events++
	r.event = event
	return r.duplicate, nil
}

var _ = errors.New
