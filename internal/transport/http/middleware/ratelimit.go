package middleware

import (
	"fmt"
	"net/http"
	"strings"
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
    EvalRequests       int // tokens per second for evaluation endpoints
    EvalBurst          int // burst capacity for evaluation endpoints
    AdminRequests      int // tokens per second for admin endpoints
    AdminBurst         int // burst capacity for admin endpoints
    ConversionRequests int // tokens per second for conversion endpoints
    ConversionBurst    int // burst capacity for conversion endpoints
}

// DefaultRateLimitConfig returns default rate limiting configuration
func DefaultRateLimitConfig() *RateLimitConfig {
    return &RateLimitConfig{
        // plan.md defaults
        EvalRequests:       50,   // 50 rps for eval
        EvalBurst:          100,  // burst 100
        AdminRequests:      10,   // 10 rps for admin
        AdminBurst:         20,   // burst 20
        ConversionRequests: 100,  // 100 rps for conversions
        ConversionBurst:    200,  // burst 200
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
    rate, burst := m.getRateAndBurst(endpointType)

		// Get or create token bucket for this tenant/endpoint combination
		bucketKey := fmt.Sprintf("%s:%s", authCtx.TenantID.String(), endpointType)
    bucket := m.getOrCreateBucket(bucketKey, burst, rate)

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
	case strings.Contains(path, "/eval"):
		return "eval"
	case strings.Contains(path, "/conversions"):
		return "conversion"
	case strings.Contains(path, "/flags") || strings.Contains(path, "/experiments"):
		return "admin"
	default:
		return "admin" // Default to admin limits for unknown endpoints
	}
}

// getRateLimit returns the rate limit for a given endpoint type
func (m *RateLimitMiddleware) getRateAndBurst(endpointType string) (rate int, burst int) {
    switch endpointType {
    case "eval":
        return m.config.EvalRequests, m.config.EvalBurst
    case "conversion":
        return m.config.ConversionRequests, m.config.ConversionBurst
    case "admin":
        return m.config.AdminRequests, m.config.AdminBurst
    default:
        return m.config.AdminRequests, m.config.AdminBurst
    }
}

// getOrCreateBucket gets an existing bucket or creates a new one
func (m *RateLimitMiddleware) getOrCreateBucket(key string, capacity int, rate int) *TokenBucket {
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

    // Create new bucket with configured capacity (burst) and refill rate (rps)
    bucket = NewTokenBucket(capacity, rate)
    m.buckets[key] = bucket

    return bucket
}

// checkGlobalRateLimit applies a global rate limit for unauthenticated requests
func (m *RateLimitMiddleware) checkGlobalRateLimit(_ *http.Request) bool {
    // For unauthenticated requests, use a global bucket with conservative limits
    bucket := m.getOrCreateBucket("global", 10, 10) // 10 rps, burst 10 globally
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
