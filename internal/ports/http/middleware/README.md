# HTTP Security Middleware

This package provides comprehensive HTTP security middleware for the FaaS feature flag service, implementing authentication, authorization, rate limiting, request validation, and idempotency handling.

## Components

### 1. HMAC Authentication Middleware (`auth.go`)

Provides HMAC-SHA256 signature verification with timestamp validation.

**Features:**
- Bearer token authentication using API key UUIDs
- HMAC-SHA256 signature verification
- Timestamp validation (5-minute window)
- Encrypted API key secret decryption
- Automatic last-used timestamp updates

**Required Headers:**
- `Authorization: Bearer <api-key-uuid>`
- `X-Timestamp: <unix-timestamp>`
- `X-Signature: <hmac-sha256-hex>`

**Signature Calculation:**
```
payload = METHOD + PATH + QUERY + TIMESTAMP + BODY_HASH
signature = HMAC-SHA256(secret, payload)
```

### 2. Request Validation Middleware (`validation.go`)

Validates and sanitizes incoming HTTP requests.

**Features:**
- Request size limits (1MB max)
- Header validation and size limits
- Path traversal protection
- JSON validation and security checks
- Body hash calculation for signature verification
- Input sanitization utilities

### 3. Idempotency Middleware (`idempotency.go`)

Handles idempotency keys for write operations to prevent duplicate processing.

**Features:**
- UUID-based idempotency keys
- Per-tenant key isolation
- Request matching validation
- Conflict detection and handling
- Optional idempotency (only for write operations)

**Usage:**
Add `Idempotency-Key: <uuid>` header to POST/PUT/PATCH/DELETE requests.

### 4. Rate Limiting Middleware (`ratelimit.go`)

Implements per-tenant rate limiting using token bucket algorithm.

**Features:**
- Per-tenant token buckets
- Different limits for endpoint types (eval, admin, conversion)
- Configurable rates and burst capacity
- Memory-efficient bucket management
- Global rate limiting for unauthenticated requests

**Default Limits:**
- Evaluation endpoints: 100 req/sec
- Admin endpoints: 10 req/sec
- Conversion endpoints: 50 req/sec

### 5. Cryptographic Support (`crypto.go`)

Provides AES-GCM decryption for API key secrets.

**Features:**
- AES-256-GCM decryption
- Environment-based key management
- Secure secret handling

**Environment Variables:**
- `API_KEY_ENCRYPTION_KEY`: 32-byte encryption key for API secrets

### 6. Middleware Chain (`chain.go`)

Provides utilities for composing middleware chains.

**Features:**
- Fluent middleware composition
- Pre-configured security chains
- Public endpoint chains (no auth)

## Usage Examples

### Basic Security Chain

```go
// Setup dependencies
apiKeyRepo := db.NewAPIKeyRepositoryAdapter(queries)
idempotencyRepo := db.NewIdempotencyRepositoryAdapter(queries)
decryptor, _ := middleware.NewAESGCMDecryptor()

// Create security chain
securityChain := middleware.SecurityMiddlewareChain(
    apiKeyRepo,
    idempotencyRepo,
    decryptor,
    nil, // use default rate limits
)

// Apply to handler
protectedHandler := securityChain.Then(yourHandler)
```

### Custom Rate Limits

```go
rateLimitConfig := &middleware.RateLimitConfig{
    EvalRequests:       200, // 200 req/sec for evaluation
    AdminRequests:      5,   // 5 req/sec for admin
    ConversionRequests: 100, // 100 req/sec for conversions
}

securityChain := middleware.SecurityMiddlewareChain(
    apiKeyRepo,
    idempotencyRepo,
    decryptor,
    rateLimitConfig,
)
```

### Public Endpoints

```go
// For endpoints that don't require authentication
publicChain := middleware.PublicMiddlewareChain(rateLimitConfig)
healthHandler := publicChain.Then(yourHealthHandler)
```

### Accessing Context

```go
func yourHandler(w http.ResponseWriter, r *http.Request) {
    // Get authentication context
    authCtx, ok := middleware.GetAuthContext(r)
    if !ok {
        http.Error(w, "Authentication required", http.StatusUnauthorized)
        return
    }
    
    // Get idempotency key if present
    idempotencyKey, hasIdempotency := middleware.GetIdempotencyKey(r)
    
    // Use tenant ID for business logic
    tenantID := authCtx.TenantID
    // ... your business logic
}
```

## Security Considerations

1. **API Key Storage**: API keys are encrypted at rest using AES-GCM
2. **Signature Verification**: All requests must include valid HMAC signatures
3. **Timestamp Validation**: Prevents replay attacks with 5-minute window
4. **Rate Limiting**: Prevents abuse with per-tenant token buckets
5. **Input Validation**: Comprehensive request validation and sanitization
6. **Path Security**: Protection against path traversal and injection attacks

## Testing

The package includes comprehensive unit tests covering:
- Valid and invalid authentication scenarios
- Rate limiting behavior under load
- Request validation edge cases
- Idempotency key handling
- Error conditions and edge cases

Run tests with:
```bash
go test ./internal/ports/http/middleware/... -v
```

## Dependencies

- `github.com/google/uuid`: UUID handling
- `github.com/stretchr/testify`: Testing framework
- Standard library: crypto, net/http, encoding, etc.

## Error Responses

All middleware components return standardized JSON error responses:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable error message"
  }
}
```

Common error codes:
- `AUTHENTICATION_FAILED`: Invalid or missing authentication
- `RATE_LIMIT_EXCEEDED`: Rate limit exceeded
- `INVALID_REQUEST_BODY`: Request validation failed
- `IDEMPOTENCY_KEY_CONFLICT`: Idempotency key conflict
- `REQUEST_TOO_LARGE`: Request exceeds size limits