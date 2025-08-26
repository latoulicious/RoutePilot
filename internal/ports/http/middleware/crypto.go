package middleware

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"os"
)

// AESGCMDecryptor implements SecretDecryptor using AES-GCM
type AESGCMDecryptor struct {
	gcm cipher.AEAD
}

// NewAESGCMDecryptor creates a new AES-GCM decryptor using KEK from environment
func NewAESGCMDecryptor() (*AESGCMDecryptor, error) {
	kek := os.Getenv("API_KEY_ENCRYPTION_KEY")
	if kek == "" {
		return nil, fmt.Errorf("API_KEY_ENCRYPTION_KEY environment variable not set")
	}

	// KEK should be 32 bytes for AES-256
	if len(kek) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes")
	}

	block, err := aes.NewCipher([]byte(kek))
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	return &AESGCMDecryptor{gcm: gcm}, nil
}

// Decrypt decrypts the encrypted data using AES-GCM
func (d *AESGCMDecryptor) Decrypt(encrypted []byte) ([]byte, error) {
	if len(encrypted) < d.gcm.NonceSize() {
		return nil, fmt.Errorf("encrypted data too short")
	}

	// Extract nonce and ciphertext
	nonce := encrypted[:d.gcm.NonceSize()]
	ciphertext := encrypted[d.gcm.NonceSize():]

	// Decrypt
	plaintext, err := d.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}

	return plaintext, nil
}