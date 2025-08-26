package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestTokenBucket_Allow(t *testing.T) {
	// Create bucket with 2 tokens, refill rate 1 token/second
	bucket := NewTokenBucket(2, 1)

	// Should allow first two requests
	assert.True(t, bucket.Allow())
	assert.True(t, bucket.Allow())

	// Third request should be denied
	assert.False(t, bucket.Allow())

	// Wait for refill and try again
	time.Sleep(1100 * time.Millisecond) // Wait slightly more than 1 second
	assert.True(t, bucket.Allow())
}

func TestTokenBucket_Refill(t *testing.T) {
	// Create bucket with 1 token, refill rate 2 tokens/second
	bucket := NewTokenBucket(1, 2)

	// Use the initial token
	assert.True(t, bucket.Allow())
	assert.False(t, bucket.Allow())

	// Wait for refill
	time.Sleep(600 * time.Millisecond) // Wait 0.6 seconds, should get 1 token
	assert.True(t, bucket.Allow())
	assert.False(t, bucket.Allow())
}

func TestTokenBucket_CapacityLimit(t *testing.T) {
	// Create bucket with capacity 2, refill rate 10 tokens/second
	bucket := NewTokenBucket(2, 10)

	// Use all tokens
	assert.True(t, bucket.Allow())
	assert.True(t, bucket.Allow())
	assert.False(t, bucket.Allow())

	// Wait long enough to refill more than capacity
	time.Sleep(1 * time.Second)

	// Should only have capacity worth of tokens
	assert.True(t, bucket.Allow())
	assert.True(t, bucket.Allow())
	assert.False(t, bucket.Allow())
}

func TestRateLimitMiddleware_AuthenticatedRequest(t *testing.T) {
	config := &RateLimitConfig{
		EvalRequests:       2,
		AdminRequests:      1,
		ConversionRequests: 1,
	}
	middleware := NewRateLimitMiddleware(config)

	tenantID := uuid.New()
	authCtx := &AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}

	// Test eval endpoint
	req := httptest.NewRequest("GET", "/v1/flags/test/eval", nil)
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(testHandler)

	// First two requests should succeed (eval limit is 2)
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Third request should be rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "RATE_LIMIT_EXCEEDED")
}

func TestRateLimitMiddleware_UnauthenticatedRequest(t *testing.T) {
	config := DefaultRateLimitConfig()
	middleware := NewRateLimitMiddleware(config)

	req := httptest.NewRequest("GET", "/health", nil)

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(testHandler)

	// Should use global rate limit (10 req/sec by default in the implementation)
	// First request should succeed
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimitMiddleware_DifferentEndpointTypes(t *testing.T) {
	config := &RateLimitConfig{
		EvalRequests:       1,
		AdminRequests:      1,
		ConversionRequests: 1,
	}
	middleware := NewRateLimitMiddleware(config)

	tenantID := uuid.New()
	authCtx := &AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}

	testCases := []struct {
		name string
		path string
	}{
		{"eval endpoint", "/v1/flags/test/eval"},
		{"admin endpoint", "/v1/flags"},
		{"conversion endpoint", "/v1/conversions"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			ctx := context.WithValue(req.Context(), "auth", authCtx)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.Middleware(testHandler)

			// First request should succeed
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)

			// Second request should be rate limited (all limits set to 1)
			rr = httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		})
	}
}

func TestRateLimitMiddleware_PerTenantIsolation(t *testing.T) {
	config := &RateLimitConfig{
		EvalRequests:       1,
		AdminRequests:      1,
		ConversionRequests: 1,
	}
	middleware := NewRateLimitMiddleware(config)

	tenant1ID := uuid.New()
	tenant2ID := uuid.New()

	authCtx1 := &AuthContext{
		TenantID: tenant1ID,
		APIKeyID: uuid.New(),
	}

	authCtx2 := &AuthContext{
		TenantID: tenant2ID,
		APIKeyID: uuid.New(),
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(testHandler)

	// Tenant 1 uses up their limit
	req1 := httptest.NewRequest("GET", "/v1/flags/test/eval", nil)
	ctx1 := context.WithValue(req1.Context(), "auth", authCtx1)
	req1 = req1.WithContext(ctx1)

	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusOK, rr1.Code)

	// Tenant 1 second request should be rate limited
	rr1 = httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusTooManyRequests, rr1.Code)

	// Tenant 2 should still be able to make requests
	req2 := httptest.NewRequest("GET", "/v1/flags/test/eval", nil)
	ctx2 := context.WithValue(req2.Context(), "auth", authCtx2)
	req2 = req2.WithContext(ctx2)

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestRateLimitMiddleware_GetEndpointType(t *testing.T) {
	middleware := NewRateLimitMiddleware(nil)

	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/flags/test/eval", "eval"},
		{"/v1/experiments/test/eval", "eval"},
		{"/v1/conversions", "conversion"},
		{"/v1/experiments/conversions", "conversion"},
		{"/v1/flags", "admin"},
		{"/v1/experiments", "admin"},
		{"/v1/unknown", "admin"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := middleware.getEndpointType(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRateLimitMiddleware_GetRateLimit(t *testing.T) {
	config := &RateLimitConfig{
		EvalRequests:       100,
		AdminRequests:      10,
		ConversionRequests: 50,
	}
	middleware := NewRateLimitMiddleware(config)

	tests := []struct {
		endpointType string
		expected     int
	}{
		{"eval", 100},
		{"admin", 10},
		{"conversion", 50},
		{"unknown", 10}, // defaults to admin
	}

	for _, tt := range tests {
		t.Run(tt.endpointType, func(t *testing.T) {
			result := middleware.getRateLimit(tt.endpointType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.Equal(t, 100, config.EvalRequests)
	assert.Equal(t, 10, config.AdminRequests)
	assert.Equal(t, 50, config.ConversionRequests)
}