package middleware_test

import (
	"fmt"
	"net/http"

	"github.com/latoulicious/RoutePilot/internal/adapters/db"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
)

// Example demonstrates how to use the security middleware chain
func ExampleSecurityMiddlewareChain() {
	// Setup dependencies (in real code, these would be properly initialized)
	var queries *db.Queries // from sqlc generated code
	
	// Create repository adapters
	apiKeyRepo := db.NewAPIKeyRepositoryAdapter(queries)
	idempotencyRepo := db.NewIdempotencyRepositoryAdapter(queries)
	
	// Create decryptor (requires API_KEY_ENCRYPTION_KEY environment variable)
	decryptor, err := middleware.NewAESGCMDecryptor()
	if err != nil {
		panic(err)
	}
	
	// Create rate limit configuration
	rateLimitConfig := &middleware.RateLimitConfig{
		EvalRequests:       100, // 100 req/sec for flag evaluation
		AdminRequests:      10,  // 10 req/sec for admin operations
		ConversionRequests: 50,  // 50 req/sec for conversion tracking
	}
	
	// Create security middleware chain
	securityChain := middleware.SecurityMiddlewareChain(
		apiKeyRepo,
		idempotencyRepo,
		decryptor,
		rateLimitConfig,
	)
	
	// Create a sample handler
	flagEvalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get authentication context
		authCtx, ok := middleware.GetAuthContext(r)
		if !ok {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		
		// Get idempotency key if present
		idempotencyKey, hasIdempotency := middleware.GetIdempotencyKey(r)
		
		fmt.Printf("Processing request for tenant: %s\n", authCtx.TenantID)
		if hasIdempotency {
			fmt.Printf("Idempotency key: %s\n", idempotencyKey)
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"enabled": true, "value": "test"}`))
	})
	
	// Apply middleware chain to handler
	protectedHandler := securityChain.Then(flagEvalHandler)
	
	// Use the protected handler in your HTTP server
	http.Handle("/v1/flags/test/eval", protectedHandler)
	
	// For public endpoints (no authentication required)
	publicChain := middleware.PublicMiddlewareChain(rateLimitConfig)
	healthHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy"}`))
	})
	
	http.Handle("/health", publicChain.Then(healthHandler))
}