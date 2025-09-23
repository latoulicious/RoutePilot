package middleware

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const AuthContextKey contextKey = "auth"

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
        ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// authenticate performs HMAC signature verification with timestamp validation
func (m *HMACAuthMiddleware) authenticate(r *http.Request) (*AuthContext, error) {
    // Extract required headers (per plan.md)
    tenantHeader := r.Header.Get("X-Tenant-ID")
    keyHeader := r.Header.Get("X-API-Key")
    timestampHeader := r.Header.Get("X-Timestamp")
    signatureHeader := r.Header.Get("X-Signature")

    if tenantHeader == "" || keyHeader == "" || timestampHeader == "" || signatureHeader == "" {
        return nil, fmt.Errorf("missing required authentication headers")
    }

    tenantID, err := uuid.Parse(tenantHeader)
    if err != nil {
        return nil, fmt.Errorf("invalid tenant ID format")
    }

    keyID, err := uuid.Parse(keyHeader)
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

    // Ensure API key belongs to the provided tenant
    if apiKey.TenantID != tenantID {
        return nil, fmt.Errorf("API key does not belong to tenant")
    }

    // Verify HMAC signature
    if err := m.verifySignature(r, secret, timestampHeader, signatureHeader); err != nil {
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
func (m *HMACAuthMiddleware) verifySignature(r *http.Request, secret []byte, timestamp, signature string) error {
    // Canonical: METHOD\nPATH_WITH_QUERY\nUNIX_TS\nSHA256_HEX(body)
    uri := r.URL.RequestURI()
    bodyHash := ""
    if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
        // Prefer hash injected by validation middleware
        bodyHash = r.Header.Get("X-Body-Hash")
        if bodyHash == "" {
            // Fallback: compute hash if not present
            b, _ := io.ReadAll(r.Body)
            sum := sha256.Sum256(b)
            bodyHash = hex.EncodeToString(sum[:])
            r.Body = io.NopCloser(strings.NewReader(string(b)))
        }
    }

    canon := strings.Join([]string{r.Method, uri, timestamp, bodyHash}, "\n")
    mac := hmac.New(sha256.New, secret)
    mac.Write([]byte(canon))
    expected := mac.Sum(nil)

    // Signature header: v1=<base64(hmac_sha256)>
    parts := strings.SplitN(signature, "=", 2)
    if len(parts) != 2 || parts[0] != "v1" {
        return errors.New("invalid signature format")
    }
    provided, err := base64.StdEncoding.DecodeString(parts[1])
    if err != nil {
        return errors.New("invalid signature encoding")
    }

    if !hmac.Equal(expected, provided) {
        return errors.New("signature mismatch")
    }
    return nil
}

// GetAuthContext extracts authentication context from request
func GetAuthContext(r *http.Request) (*AuthContext, bool) {
    auth, ok := r.Context().Value(AuthContextKey).(*AuthContext)
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
