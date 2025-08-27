package middleware

import (
    "net/http"
)

// Chain represents a middleware chain
type Chain struct {
    middlewares []func(http.Handler) http.Handler
}

// NewChain creates a new middleware chain
func NewChain(middlewares ...func(http.Handler) http.Handler) *Chain {
    return &Chain{
        middlewares: middlewares,
    }
}

// Then applies the middleware chain to a handler
func (c *Chain) Then(handler http.Handler) http.Handler {
    // Apply middlewares in reverse order so they execute in the correct order
    for i := len(c.middlewares) - 1; i >= 0; i-- {
        handler = c.middlewares[i](handler)
    }
    return handler
}

// Append adds middleware to the end of the chain
func (c *Chain) Append(middlewares ...func(http.Handler) http.Handler) *Chain {
    newMiddlewares := make([]func(http.Handler) http.Handler, len(c.middlewares)+len(middlewares))
    copy(newMiddlewares, c.middlewares)
    copy(newMiddlewares[len(c.middlewares):], middlewares)
    
    return &Chain{
        middlewares: newMiddlewares,
    }
}

// SecurityMiddlewareChain creates a standard security middleware chain
func SecurityMiddlewareChain(
    apiKeyRepo APIKeyRepository,
    idempotencyRepo IdempotencyRepository,
    decryptor SecretDecryptor,
    rateLimitConfig *RateLimitConfig,
) *Chain {
    validation := NewValidationMiddleware()
    auth := NewHMACAuthMiddleware(apiKeyRepo, decryptor)
    idempotency := NewIdempotencyMiddleware(idempotencyRepo)
    rateLimit := NewRateLimitMiddleware(rateLimitConfig)

    return NewChain(
        // Validate and hash body first so auth can use X-Body-Hash
        validation.Middleware,
        // Authenticate to identify tenant before applying per-tenant rate limits
        auth.Middleware,
        // Apply per-tenant rate limits
        rateLimit.Middleware,
        // Enforce idempotency for write operations
        idempotency.Middleware,
    )
}

// PublicMiddlewareChain creates a middleware chain for public endpoints (no auth)
func PublicMiddlewareChain(rateLimitConfig *RateLimitConfig) *Chain {
    validation := NewValidationMiddleware()
    rateLimit := NewRateLimitMiddleware(rateLimitConfig)

    return NewChain(
        validation.Middleware,
        rateLimit.Middleware,
    )
}
