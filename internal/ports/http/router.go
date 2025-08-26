package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
)

// FlagRuleRepository defines the interface for flag rule operations
type FlagRuleRepository interface {
	CreateFlagRule(ctx context.Context, rule *flags.FlagRule) error
}

// CacheInvalidator defines the interface for cache invalidation
type CacheInvalidator interface {
	InvalidateFlag(tenantID uuid.UUID, flagKey string)
}

// RouterConfig holds configuration for the HTTP router
type RouterConfig struct {
	Evaluator       flags.Evaluator
	OutboxRepo      ports.OutboxRepository
	FlagRepo        ports.FlagRepository
	FlagRuleRepo    FlagRuleRepository
	ExperimentRepo  ports.ExperimentRepository
	Cache           CacheInvalidator
	APIKeyRepo      middleware.APIKeyRepository
	IdempotencyRepo middleware.IdempotencyRepository
	Decryptor       middleware.SecretDecryptor
	RateLimitConfig *middleware.RateLimitConfig
}

// NewRouter creates a new HTTP router with all routes and middleware configured
func NewRouter(config RouterConfig) *mux.Router {
	router := mux.NewRouter()

	// Create handlers
	flagHandler := NewFlagHandler(config.Evaluator, config.OutboxRepo)
	adminHandler := NewAdminHandler(config.FlagRepo, config.FlagRuleRepo, config.Cache)
	experimentHandler := NewExperimentHandler(config.ExperimentRepo, config.FlagRepo, config.Cache)

	// Create middleware chains
	securityChain := middleware.SecurityMiddlewareChain(
		config.APIKeyRepo,
		config.IdempotencyRepo,
		config.Decryptor,
		config.RateLimitConfig,
	)

	// API v1 routes
	v1 := router.PathPrefix("/v1").Subrouter()

	// Flag evaluation endpoint (GET /v1/flags/{key}/eval)
	v1.Handle("/flags/{key}/eval", 
		securityChain.Then(http.HandlerFunc(flagHandler.EvaluateFlag))).
		Methods("GET")

	// Conversion tracking endpoint (POST /v1/experiments/conversions)
	v1.Handle("/experiments/conversions", 
		securityChain.Then(http.HandlerFunc(flagHandler.TrackConversion))).
		Methods("POST")

	// Admin endpoints for flag management
	// Create flag endpoint (POST /v1/flags)
	v1.Handle("/flags", 
		securityChain.Then(http.HandlerFunc(adminHandler.CreateFlag))).
		Methods("POST")

	// Create flag rule endpoint (POST /v1/flags/{key}/rules)
	v1.Handle("/flags/{key}/rules", 
		securityChain.Then(http.HandlerFunc(adminHandler.CreateFlagRule))).
		Methods("POST")

	// Update flag endpoint (PATCH /v1/flags/{key})
	v1.Handle("/flags/{key}", 
		securityChain.Then(http.HandlerFunc(adminHandler.UpdateFlag))).
		Methods("PATCH")

	// Experiment management endpoints
	// Create experiment endpoint (POST /v1/experiments)
	v1.Handle("/experiments", 
		securityChain.Then(http.HandlerFunc(experimentHandler.CreateExperiment))).
		Methods("POST")

	// Create experiment variant endpoint (POST /v1/experiments/{key}/variants)
	v1.Handle("/experiments/{key}/variants", 
		securityChain.Then(http.HandlerFunc(experimentHandler.CreateVariant))).
		Methods("POST")

	// Update experiment endpoint (PATCH /v1/experiments/{key})
	v1.Handle("/experiments/{key}", 
		securityChain.Then(http.HandlerFunc(experimentHandler.UpdateExperiment))).
		Methods("PATCH")

	// Health check endpoint (no authentication required)
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")

	return router
}

// NewTestRouter creates a router for testing with minimal middleware
func NewTestRouter(evaluator flags.Evaluator, outboxRepo ports.OutboxRepository) *mux.Router {
	router := mux.NewRouter()

	// Create handlers
	flagHandler := NewFlagHandler(evaluator, outboxRepo)

	// API v1 routes with minimal middleware for testing
	v1 := router.PathPrefix("/v1").Subrouter()

	// Add a simple test middleware that adds mock auth context
	testAuthMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add mock auth context
			authCtx := &middleware.AuthContext{
				TenantID: uuid.New(),
				APIKeyID: uuid.New(),
			}
			ctx := context.WithValue(r.Context(), "auth", authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Flag evaluation endpoint with test middleware
	v1.Handle("/flags/{key}/eval", 
		testAuthMiddleware(http.HandlerFunc(flagHandler.EvaluateFlag))).
		Methods("GET")

	// Conversion tracking endpoint with test middleware
	v1.Handle("/experiments/conversions", 
		testAuthMiddleware(http.HandlerFunc(flagHandler.TrackConversion))).
		Methods("POST")

	// Health check endpoint
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")

	return router
}

// healthCheckHandler provides a simple health check endpoint
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"faas-api"}`))
}