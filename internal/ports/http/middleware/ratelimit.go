package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	capacity   int
	tokens     int
	refillRate int // tokens per second
	lastRefill time.Time
	mutex      sync.Mutex
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed and consumes a token if so
func (tb *TokenBucket) Allow() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed * float64(tb.refillRate))
	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}

	// Check if we have tokens available
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// RateLimitConfig defines rate limiting configuration for different endpoints
type RateLimitConfig struct {
	EvalRequests       int // requests per second for evaluation endpoints
	AdminRequests      int // requests per second for admin endpoints
	ConversionRequests int // requests per second for conversion endpoints
}

// DefaultRateLimitConfig returns default rate limiting configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		EvalRequests:       100, // 100 req/sec for flag evaluation
		AdminRequests:      10,  // 10 req/sec for admin operations
		ConversionRequests: 50,  // 50 req/sec for conversion tracking
	}
}

// RateLimitMiddleware provides per-tenant rate limiting using token bucket algorithm
type RateLimitMiddleware struct {
	config  *RateLimitConfig
	buckets map[string]*TokenBucket // key: tenantID:endpoint_type
	mutex   sync.RWMutex
}

// NewRateLimitMiddleware creates a new rate limiting middleware
func NewRateLimitMiddleware(config *RateLimitConfig) *RateLimitMiddleware {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	return &RateLimitMiddleware{
		config:  config,
		buckets: make(map[string]*TokenBucket),
	}
}

// Middleware returns the HTTP middleware function
func (m *RateLimitMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get auth context for tenant ID
		authCtx, ok := GetAuthContext(r)
		if !ok {
			// If no auth context, apply global rate limiting
			if !m.checkGlobalRateLimit(r) {
				m.writeRateLimitResponse(w)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Determine endpoint type and rate limit
		endpointType := m.getEndpointType(r.URL.Path)
		limit := m.getRateLimit(endpointType)

		// Get or create token bucket for this tenant/endpoint combination
		bucketKey := fmt.Sprintf("%s:%s", authCtx.TenantID.String(), endpointType)
		bucket := m.getOrCreateBucket(bucketKey, limit)

		// Check if request is allowed
		if !bucket.Allow() {
			m.writeRateLimitResponse(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getEndpointType determines the type of endpoint based on the path
func (m *RateLimitMiddleware) getEndpointType(path string) string {
	switch {
	case contains(path, "/eval"):
		return "eval"
	case contains(path, "/conversions"):
		return "conversion"
	case contains(path, "/flags") || contains(path, "/experiments"):
		return "admin"
	default:
		return "admin" // Default to admin limits for unknown endpoints
	}
}

// getRateLimit returns the rate limit for a given endpoint type
func (m *RateLimitMiddleware) getRateLimit(endpointType string) int {
	switch endpointType {
	case "eval":
		return m.config.EvalRequests
	case "conversion":
		return m.config.ConversionRequests
	case "admin":
		return m.config.AdminRequests
	default:
		return m.config.AdminRequests
	}
}

// getOrCreateBucket gets an existing bucket or creates a new one
func (m *RateLimitMiddleware) getOrCreateBucket(key string, limit int) *TokenBucket {
	m.mutex.RLock()
	bucket, exists := m.buckets[key]
	m.mutex.RUnlock()

	if exists {
		return bucket
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists := m.buckets[key]; exists {
		return bucket
	}

	// Create new bucket with capacity equal to limit and refill rate equal to limit
	bucket = NewTokenBucket(limit, limit)
	m.buckets[key] = bucket

	return bucket
}

// checkGlobalRateLimit applies a global rate limit for unauthenticated requests
func (m *RateLimitMiddleware) checkGlobalRateLimit(_ *http.Request) bool {
	// For unauthenticated requests, use a global bucket with conservative limits
	bucket := m.getOrCreateBucket("global", 10) // 10 req/sec globally
	return bucket.Allow()
}

// writeRateLimitResponse writes a rate limit exceeded response
func (m *RateLimitMiddleware) writeRateLimitResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1") // Suggest retry after 1 second
	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprintf(w, `{"error":{"code":"RATE_LIMIT_EXCEEDED","message":"Rate limit exceeded, please retry later"}}`)
}

// CleanupExpiredBuckets removes unused buckets to prevent memory leaks
func (m *RateLimitMiddleware) CleanupExpiredBuckets() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// In a production system, you'd track last access time and remove old buckets
	// For now, this is a placeholder for the cleanup logic
	// You could run this periodically in a goroutine
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					indexOf(s, substr) >= 0)))
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
