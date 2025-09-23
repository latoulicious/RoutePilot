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
    UpdateIdempotencyStatus(ctx context.Context, tenantID, key uuid.UUID, status int32) error
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
            // Enforce presence for write operations per plan.md
            writeErrorResponse(w, http.StatusBadRequest, "MISSING_IDEMPOTENCY_KEY", "Idempotency-Key header is required for write operations")
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

        // Wrap ResponseWriter to capture status for update
        ctx := context.WithValue(r.Context(), idempotencyContextKey, idempotencyKey)
        rw := &statusCapturingWriter{ResponseWriter: w, status: 0}
        next.ServeHTTP(rw, r.WithContext(ctx))
        // After handler runs, update stored status
        if rw.status == 0 {
            rw.status = http.StatusOK
        }
        _ = m.repo.UpdateIdempotencyStatus(r.Context(), authCtx.TenantID, idempotencyKey, int32(rw.status))
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
    return m.repo.UpdateIdempotencyStatus(ctx, tenantID, key, int32(status))
}

// statusCapturingWriter captures response status code
type statusCapturingWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusCapturingWriter) WriteHeader(statusCode int) {
    w.status = statusCode
    w.ResponseWriter.WriteHeader(statusCode)
}
