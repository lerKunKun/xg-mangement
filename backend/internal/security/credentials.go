package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const credentialCipherVersion byte = 1

type CredentialCipher struct {
	aead cipher.AEAD
}

func NewCredentialCipher(encodedKey string) (*CredentialCipher, error) {
	key := []byte(encodedKey)
	if decoded, err := base64.StdEncoding.DecodeString(encodedKey); err == nil && len(decoded) == 32 {
		key = decoded
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("credential encryption key must be exactly 32 bytes or base64-encoded 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &CredentialCipher{aead: aead}, nil
}

func (c *CredentialCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate credential nonce: %w", err)
	}
	result := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	result[0] = credentialCipherVersion
	result = append(result, nonce...)
	result = c.aead.Seal(result, nonce, plaintext, []byte{credentialCipherVersion})
	return result, nil
}

func (c *CredentialCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	minimum := 1 + c.aead.NonceSize() + c.aead.Overhead()
	if len(ciphertext) < minimum || ciphertext[0] != credentialCipherVersion {
		return nil, fmt.Errorf("unsupported or malformed credential ciphertext")
	}
	nonceEnd := 1 + c.aead.NonceSize()
	plaintext, err := c.aead.Open(nil, ciphertext[1:nonceEnd], ciphertext[nonceEnd:], []byte{credentialCipherVersion})
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}
	return plaintext, nil
}
