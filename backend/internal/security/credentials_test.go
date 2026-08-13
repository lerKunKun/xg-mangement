package security

import (
	"bytes"
	"testing"
)

func TestCredentialCipherRoundTripAndRandomNonce(t *testing.T) {
	cipher, err := NewCredentialCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewCredentialCipher: %v", err)
	}

	plaintext := []byte(`{"client_secret":"never-return-this"}`)
	first, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt first: %v", err)
	}
	second, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt second: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("ciphertexts are equal; want a fresh nonce")
	}
	if bytes.Contains(first, []byte("never-return-this")) {
		t.Fatal("ciphertext contains plaintext")
	}

	decrypted, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestCredentialCipherRejectsShortKeyAndTampering(t *testing.T) {
	if _, err := NewCredentialCipher("too-short"); err == nil {
		t.Fatal("short key accepted")
	}

	cipher, err := NewCredentialCipher("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewCredentialCipher: %v", err)
	}
	ciphertext, err := cipher.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 0xff
	if _, err := cipher.Decrypt(ciphertext); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}
