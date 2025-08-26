package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuthContext holds authentication information for the request
type AuthContext struct {
	TenantID uuid.UUID
	APIKeyID uuid.UUID
}

// APIKeyRepository defines the interface for API key operations
type APIKeyRepository interface {
	GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (*APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error
}

// APIKey represents an API key from the database
type APIKey struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	SecretEnc []byte
	Active    bool
	CreatedAt time.Time
}

// HMACAuthMiddleware provides HMAC signature verification with timestamp validation
type HMACAuthMiddleware struct {
	apiKeyRepo APIKeyRepository
	decryptor  SecretDecryptor
}

// SecretDecryptor interface for decrypting API key secrets
type SecretDecryptor interface {
	Decrypt(encrypted []byte) ([]byte, error)
}

// NewHMACAuthMiddleware creates a new HMAC authentication middleware
func NewHMACAuthMiddleware(apiKeyRepo APIKeyRepository, decryptor SecretDecryptor) *HMACAuthMiddleware {
	return &HMACAuthMiddleware{
		apiKeyRepo: apiKeyRepo,
		decryptor:  decryptor,
	}
}

// Middleware returns the HTTP middleware function
func (m *HMACAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx, err := m.authenticate(r)
		if err != nil {
			writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_FAILED", err.Error())
			return
		}

		// Add auth context to request
		ctx := context.WithValue(r.Context(), "auth", authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate performs HMAC signature verification with timestamp validation
func (m *HMACAuthMiddleware) authenticate(r *http.Request) (*AuthContext, error) {
	// Extract required headers
	authHeader := r.Header.Get("Authorization")
	timestampHeader := r.Header.Get("X-Timestamp")
	signatureHeader := r.Header.Get("X-Signature")

	if authHeader == "" || timestampHeader == "" || signatureHeader == "" {
		return nil, fmt.Errorf("missing required authentication headers")
	}

	// Parse API key from Authorization header (Bearer format)
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("invalid authorization header format")
	}
	
	keyIDStr := strings.TrimPrefix(authHeader, "Bearer ")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid API key format")
	}

	// Validate timestamp (within 5-minute window)
	timestamp, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format")
	}

	now := time.Now().Unix()
	if abs(now-timestamp) > 300 { // 5 minutes
		return nil, fmt.Errorf("timestamp outside valid window")
	}

	// Get API key from database
	apiKey, err := m.apiKeyRepo.GetAPIKeyByID(r.Context(), keyID)
	if err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	if !apiKey.Active {
		return nil, fmt.Errorf("API key is inactive")
	}

	// Decrypt the secret
	secret, err := m.decryptor.Decrypt(apiKey.SecretEnc)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt API key secret")
	}

	// Verify HMAC signature
	if !m.verifySignature(r, string(secret), timestampHeader, signatureHeader) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Update last used timestamp (async, don't block on errors)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.apiKeyRepo.UpdateAPIKeyLastUsed(ctx, keyID)
	}()

	return &AuthContext{
		TenantID: apiKey.TenantID,
		APIKeyID: keyID,
	}, nil
}

// verifySignature verifies the HMAC-SHA256 signature
func (m *HMACAuthMiddleware) verifySignature(r *http.Request, secret, timestamp, signature string) bool {
	// Create signature payload: METHOD + PATH + QUERY + TIMESTAMP + BODY
	payload := fmt.Sprintf("%s%s%s%s", 
		r.Method, 
		r.URL.Path, 
		r.URL.RawQuery, 
		timestamp,
	)

	// For POST/PUT requests, include body in signature
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		// Note: In a real implementation, you'd need to read and restore the body
		// This is a simplified version - body reading would be handled by another middleware
		if body := r.Header.Get("X-Body-Hash"); body != "" {
			payload += body
		}
	}

	// Calculate HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// GetAuthContext extracts authentication context from request
func GetAuthContext(r *http.Request) (*AuthContext, bool) {
	auth, ok := r.Context().Value("auth").(*AuthContext)
	return auth, ok
}

// abs returns the absolute value of an integer
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// writeErrorResponse writes a standardized error response
func writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":"%s","message":"%s"}}`, code, message)
}