package shopify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

func VerifyWebhookHMAC(rawBody []byte, signature, clientSecret string) bool {
	provided, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || len(provided) == 0 || clientSecret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(clientSecret))
	_, _ = mac.Write(rawBody)
	return hmac.Equal(provided, mac.Sum(nil))
}
