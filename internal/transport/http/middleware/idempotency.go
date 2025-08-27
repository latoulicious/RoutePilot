package middleware

import (
    "context"
    "crypto/sha256"
    "fmt"
    "net/http"
    "time"

    "github.com/google/uuid"
)

const idempotencyContextKey contextKey = "idempotency_key"

// IdempotencyRepository defines the interface for idempotency key operations
type IdempotencyRepository interface {
    GetIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID) (*IdempotencyKey, error)
    CreateIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID, method string, pathHash []byte, status int32) (*IdempotencyKey, error)
}

// IdempotencyKey represents an idempotency key from the database
type IdempotencyKey struct {
    TenantID  uuid.UUID
    Key       uuid.UUID
    Method    string
    PathHash  []byte
    Status    int32
    CreatedAt time.Time
}

// IdempotencyMiddleware handles idempotency key processing for write operations
type IdempotencyMiddleware struct {
    repo IdempotencyRepository
}

// NewIdempotencyMiddleware creates a new idempotency middleware
func NewIdempotencyMiddleware(repo IdempotencyRepository) *IdempotencyMiddleware {
    return &IdempotencyMiddleware{
        repo: repo,
    }
}

// Middleware returns the HTTP middleware function for write operations
func (m *IdempotencyMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Only apply to write operations
        if r.Method != "POST" && r.Method != "PUT" && r.Method != "PATCH" && r.Method != "DELETE" {
            next.ServeHTTP(w, r)
            return
        }

        // Get idempotency key from header
        idempotencyKeyHeader := r.Header.Get("Idempotency-Key")
        if idempotencyKeyHeader == "" {
            // Idempotency key is optional, continue without it
            next.ServeHTTP(w, r)
            return
        }

        // Parse idempotency key
        idempotencyKey, err := uuid.Parse(idempotencyKeyHeader)
        if err != nil {
            writeErrorResponse(w, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency key must be a valid UUID")
            return
        }

        // Get auth context
        authCtx, ok := GetAuthContext(r)
        if !ok {
            writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication required for idempotency")
            return
        }

        // Create path hash for uniqueness
        pathHash := m.createPathHash(r.Method, r.URL.Path, r.URL.RawQuery)

        // Check if idempotency key already exists
        existingKey, err := m.repo.GetIdempotencyKey(r.Context(), authCtx.TenantID, idempotencyKey)
        if err == nil {
            // Key exists, check if it matches current request
            if !m.requestMatches(existingKey, r.Method, pathHash) {
                writeErrorResponse(w, http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT",
                    "Idempotency key already used for different request")
                return
            }

            // Return the previous response status
            w.WriteHeader(int(existingKey.Status))
            fmt.Fprintf(w, `{"message":"Request already processed","idempotency_key":"%s"}`, idempotencyKey)
            return
        }

        // Create new idempotency key record (status will be updated after processing)
        _, err = m.repo.CreateIdempotencyKey(r.Context(), authCtx.TenantID, idempotencyKey, r.Method, pathHash, 0)
        if err != nil {
            // If creation fails due to conflict, another request is processing
            writeErrorResponse(w, http.StatusConflict, "IDEMPOTENCY_KEY_PROCESSING",
                "Request with this idempotency key is currently being processed")
            return
        }

        // Add idempotency context to request
        ctx := context.WithValue(r.Context(), idempotencyContextKey, idempotencyKey)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// createPathHash creates a hash of the request path and query for uniqueness checking
func (m *IdempotencyMiddleware) createPathHash(method, path, query string) []byte {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%s:%s:%s", method, path, query)))
    return h.Sum(nil)
}

// requestMatches checks if the existing idempotency key matches the current request
func (m *IdempotencyMiddleware) requestMatches(existing *IdempotencyKey, method string, pathHash []byte) bool {
    if existing.Method != method {
        return false
    }

    if len(existing.PathHash) != len(pathHash) {
        return false
    }

    for i, b := range existing.PathHash {
        if b != pathHash[i] {
            return false
        }
    }

    return true
}

// GetIdempotencyKey extracts idempotency key from request context
func GetIdempotencyKey(r *http.Request) (uuid.UUID, bool) {
    key, ok := r.Context().Value(idempotencyContextKey).(uuid.UUID)
    return key, ok
}

// UpdateIdempotencyStatus updates the status of an idempotency key after processing
func (m *IdempotencyMiddleware) UpdateIdempotencyStatus(ctx context.Context, tenantID, key uuid.UUID, status int) error {
    // This would typically be called by the handler after successful processing
    // For now, we'll leave this as a placeholder since we don't have an update query
    // In a real implementation, you'd add an UPDATE query to the SQL files
    return nil
}

