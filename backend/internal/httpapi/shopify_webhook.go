package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/integrations/shopify"
)

const maxShopifyWebhookBytes = 2 << 20

var ErrShopifyWebhookStoreNotFound = errors.New("Shopify webhook store not found")

type SecretDecryptor interface {
	Decrypt([]byte) ([]byte, error)
}

type ShopifyWebhookTarget struct {
	OrganizationID   string
	StoreID          string
	EncryptedSecrets []byte
}

type ShopifyWebhookEvent struct {
	OrganizationID string
	StoreID        string
	WebhookID      string
	ShopDomain     string
	Topic          string
	Body           []byte
}

type ShopifyWebhookRepository interface {
	ResolveShopifyWebhookTarget(context.Context, string) (ShopifyWebhookTarget, error)
	RecordShopifyEventAndOutbox(context.Context, ShopifyWebhookEvent) (bool, error)
}

type ShopifyWebhookDependencies struct {
	Repository ShopifyWebhookRepository
	Cipher     SecretDecryptor
}

func shopifyWebhook(deps ShopifyWebhookDependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		webhookID := strings.TrimSpace(c.GetHeader("X-Shopify-Webhook-Id"))
		domain := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Shopify-Shop-Domain")))
		topic := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Shopify-Topic")))
		signature := strings.TrimSpace(c.GetHeader("X-Shopify-Hmac-Sha256"))
		if webhookID == "" || domain == "" || topic == "" || signature == "" {
			respondError(c, http.StatusBadRequest, "invalid_webhook_headers", "Required Shopify webhook headers are missing.")
			return
		}
		if _, err := shopify.NormalizeShopDomain(domain); err != nil {
			respondError(c, http.StatusBadRequest, "invalid_shop_domain", "The Shopify shop domain is invalid.")
			return
		}
		target, err := deps.Repository.ResolveShopifyWebhookTarget(c.Request.Context(), domain)
		if errors.Is(err, ErrShopifyWebhookStoreNotFound) {
			respondError(c, http.StatusNotFound, "webhook_store_not_found", "The Shopify store is not connected.")
			return
		}
		if err != nil {
			respondError(c, http.StatusInternalServerError, "internal_error", "The Shopify webhook could not be resolved.")
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxShopifyWebhookBytes+1))
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_webhook_body", "The Shopify webhook body could not be read.")
			return
		}
		if len(body) > maxShopifyWebhookBytes {
			respondError(c, http.StatusRequestEntityTooLarge, "webhook_too_large", "The Shopify webhook body exceeds 2 MiB.")
			return
		}
		secret, err := webhookClientSecret(deps.Cipher, target.EncryptedSecrets)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "webhook_secret_unavailable", "The Shopify webhook configuration is unavailable.")
			return
		}
		if !shopify.VerifyWebhookHMAC(body, signature, secret) {
			respondError(c, http.StatusUnauthorized, "invalid_webhook_signature", "The Shopify webhook signature is invalid.")
			return
		}
		duplicate, err := deps.Repository.RecordShopifyEventAndOutbox(c.Request.Context(), ShopifyWebhookEvent{
			OrganizationID: target.OrganizationID, StoreID: target.StoreID, WebhookID: webhookID,
			ShopDomain: domain, Topic: topic, Body: append([]byte(nil), body...),
		})
		if err != nil {
			respondError(c, http.StatusInternalServerError, "webhook_persist_failed", "The Shopify webhook could not be recorded.")
			return
		}
		if duplicate {
			respondData(c, http.StatusOK, gin.H{"duplicate": true})
			return
		}
		respondData(c, http.StatusAccepted, gin.H{"accepted": true})
	}
}

func webhookClientSecret(cipher SecretDecryptor, encrypted []byte) (string, error) {
	if cipher == nil {
		return "", fmt.Errorf("webhook credential cipher is missing")
	}
	plaintext, err := cipher.Decrypt(encrypted)
	if err != nil {
		return "", err
	}
	var secrets struct {
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		return "", err
	}
	if strings.TrimSpace(secrets.ClientSecret) == "" {
		return "", fmt.Errorf("Shopify client secret is empty")
	}
	return secrets.ClientSecret, nil
}
