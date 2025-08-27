package middleware

import (
    "crypto/aes"
    "crypto/cipher"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "os"
)

// AESGCMDecryptor implements SecretDecryptor using AES-GCM
type AESGCMDecryptor struct {
    gcm cipher.AEAD
}

// NewAESGCMDecryptor creates a new AES-GCM decryptor using KEK from environment
func NewAESGCMDecryptor() (*AESGCMDecryptor, error) {
    raw := os.Getenv("KEK")
    if raw == "" {
        raw = os.Getenv("API_KEY_ENCRYPTION_KEY")
    }
    if raw == "" {
        return nil, fmt.Errorf("KEK/API_KEY_ENCRYPTION_KEY environment variable not set")
    }

    // Accept raw 32 bytes, or hex/base64 encodings
    key := []byte(raw)
    if len(key) != 32 {
        if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
            key = b
        } else if b2, err2 := base64.StdEncoding.DecodeString(raw); err2 == nil && len(b2) == 32 {
            key = b2
        } else {
            return nil, fmt.Errorf("KEK must be 32 bytes raw, hex, or base64")
        }
    }

    block, err := aes.NewCipher(key)
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
