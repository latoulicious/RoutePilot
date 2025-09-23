package cli

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
    "errors"
    "fmt"
    "os"
)

// loadKEK loads the encryption key from env (prefer KEK, fallback API_KEY_ENCRYPTION_KEY)
func loadKEK() ([]byte, error) {
    // Prefer KEK, fallback to API_KEY_ENCRYPTION_KEY
    if raw := os.Getenv("KEK"); raw != "" {
        if key, err := decodeKey(raw); err == nil {
            return key, nil
        } else {
            return nil, err
        }
    }
    if raw := os.Getenv("API_KEY_ENCRYPTION_KEY"); raw != "" {
        if key, err := decodeKey(raw); err == nil {
            return key, nil
        } else {
            return nil, err
        }
    }
    return nil, errors.New("KEK not set; please export KEK or API_KEY_ENCRYPTION_KEY (32-byte raw, hex, or base64)")
}

// decodeKey accepts raw 32-byte string, or hex/base64 encodings that decode to 32 bytes
func decodeKey(raw string) ([]byte, error) {
    // Raw 32 bytes (as-is) support
    if len(raw) == 32 {
        return []byte(raw), nil
    }
    // Hex
    if b, err := hex.DecodeString(raw); err == nil && len(b) == 32 {
        return b, nil
    }
    // Base64 (std)
    if b, err := base64.StdEncoding.DecodeString(raw); err == nil && len(b) == 32 {
        return b, nil
    }
    return nil, fmt.Errorf("invalid KEK length/encoding; must be 32-byte raw, hex, or base64")
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
