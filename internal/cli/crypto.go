package cli

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "errors"
    "fmt"
    "os"
)

// loadKEK loads the encryption key from env (prefer KEK, fallback API_KEY_ENCRYPTION_KEY)
func loadKEK() ([]byte, error) {
    if v := os.Getenv("KEK"); v != "" {
        if len(v) == 32 { // raw 32-byte string
            return []byte(v), nil
        }
        return nil, fmt.Errorf("KEK must be 32 bytes for AES-256-GCM")
    }
    if v := os.Getenv("API_KEY_ENCRYPTION_KEY"); v != "" {
        if len(v) == 32 {
            return []byte(v), nil
        }
        return nil, fmt.Errorf("API_KEY_ENCRYPTION_KEY must be 32 bytes")
    }
    return nil, errors.New("KEK not set; please export KEK or API_KEY_ENCRYPTION_KEY (32 bytes)")
}

// encryptAESGCM encrypts plaintext with AES-GCM using key; returns nonce||ciphertext
func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return nil, err
    }
    sealed := gcm.Seal(nil, nonce, plaintext, nil)
    out := make([]byte, 0, len(nonce)+len(sealed))
    out = append(out, nonce...)
    out = append(out, sealed...)
    return out, nil
}

