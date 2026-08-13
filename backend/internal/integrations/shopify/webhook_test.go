package shopify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestVerifyWebhookHMAC(t *testing.T) {
	body := []byte(`{"id":1}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !VerifyWebhookHMAC(body, signature, "secret") {
		t.Fatal("valid signature rejected")
	}
	if VerifyWebhookHMAC([]byte(`{"id":2}`), signature, "secret") {
		t.Fatal("tampered body accepted")
	}
}
