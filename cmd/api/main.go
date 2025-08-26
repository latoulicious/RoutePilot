package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	httpPort "github.com/latoulicious/RoutePilot/internal/ports/http"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
)

func main() {
	fmt.Println("Starting FaaS API Server...")

	// Initialize mock repositories for demonstration
	// In a real implementation, these would be proper database repositories
	flagRepo := &mockFlagRepo{}
	assignmentRepo := &mockAssignmentRepo{}
	experimentRepo := &mockExperimentRepo{}
	outboxRepo := &mockOutboxRepo{}

	// Initialize core services
	experimentEngine := flags.NewExperimentEngine(experimentRepo, assignmentRepo)
	evaluator := flags.NewFlagEvaluator(flagRepo, assignmentRepo, experimentRepo, experimentEngine)

	// Initialize middleware dependencies (mocked for now)
	rateLimitConfig := middleware.DefaultRateLimitConfig()

	// Create HTTP router with minimal middleware for now
	// In a real implementation, you'd properly initialize all middleware
	routerConfig := httpPort.RouterConfig{
		Evaluator:       evaluator,
		OutboxRepo:      outboxRepo,
		APIKeyRepo:      &mockAPIKeyRepo{},
		IdempotencyRepo: &mockIdempotencyRepo{},
		Decryptor:       &mockDecryptor{},
		RateLimitConfig: rateLimitConfig,
	}

	router := httpPort.NewRouter(routerConfig)

	// Create and start HTTP server
	serverConfig := httpPort.DefaultServerConfig()
	server := httpPort.NewServer(router, serverConfig)

	// Start server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	fmt.Println("Server exited")
}

// Mock implementations for middleware dependencies
// These would be replaced with real implementations in production

type mockAPIKeyRepo struct{}

func (m *mockAPIKeyRepo) GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (*middleware.APIKey, error) {
	// Return a mock API key for testing
	return &middleware.APIKey{
		ID:        keyID,
		TenantID:  uuid.New(),
		Name:      "test-key",
		SecretEnc: []byte("mock-secret"),
		Active:    true,
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockAPIKeyRepo) UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error {
	return nil
}

type mockIdempotencyRepo struct{}

func (m *mockIdempotencyRepo) GetIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID) (*middleware.IdempotencyKey, error) {
	return nil, fmt.Errorf("not found")
}

func (m *mockIdempotencyRepo) CreateIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID, method string, pathHash []byte, status int32) (*middleware.IdempotencyKey, error) {
	return &middleware.IdempotencyKey{
		TenantID:  tenantID,
		Key:       key,
		Method:    method,
		PathHash:  pathHash,
		Status:    status,
		CreatedAt: time.Now(),
	}, nil
}

type mockDecryptor struct{}

func (m *mockDecryptor) Decrypt(encrypted []byte) ([]byte, error) {
	// Return the encrypted data as-is for testing
	return encrypted, nil
}

// Mock repository implementations for demonstration
type mockFlagRepo struct{}

func (m *mockFlagRepo) GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*flags.Flag, error) {
	// Return a mock flag for demonstration
	return &flags.Flag{
		ID:          uuid.New(),
		TenantID:    tenantID,
		Key:         key,
		Description: "Demo flag",
		Type:        flags.FlagTypeBoolean,
		Enabled:     true,
		Salt:        "demo-salt",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (m *mockFlagRepo) GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*flags.FlagRule, error) {
	// Return a mock rule with 50% rollout
	return []*flags.FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  50,
			Variant:  json.RawMessage(`true`),
		},
	}, nil
}

func (m *mockFlagRepo) CreateFlag(ctx context.Context, flag *flags.Flag) error {
	return nil
}

func (m *mockFlagRepo) UpdateFlag(ctx context.Context, flagID uuid.UUID, updates flags.FlagUpdates) error {
	return nil
}

func (m *mockFlagRepo) DeleteFlag(ctx context.Context, flagID uuid.UUID) error {
	return nil
}

type mockAssignmentRepo struct{}

func (m *mockAssignmentRepo) GetAssignment(ctx context.Context, flagID uuid.UUID, subjectID string) (*flags.Assignment, error) {
	// Return no existing assignment to test new assignment creation
	return nil, fmt.Errorf("assignment not found")
}

func (m *mockAssignmentRepo) UpsertAssignment(ctx context.Context, assignment *flags.Assignment) error {
	return nil
}

type mockExperimentRepo struct{}

func (m *mockExperimentRepo) GetExperimentByFlagID(ctx context.Context, flagID uuid.UUID) (*flags.Experiment, error) {
	// Return no experiment for simplicity
	return nil, fmt.Errorf("experiment not found")
}

func (m *mockExperimentRepo) GetExperimentVariants(ctx context.Context, experimentID uuid.UUID) ([]*flags.ExperimentVariant, error) {
	return nil, nil
}

func (m *mockExperimentRepo) CreateExperiment(ctx context.Context, experiment *flags.Experiment) error {
	return nil
}

func (m *mockExperimentRepo) UpdateExperiment(ctx context.Context, experimentID uuid.UUID, status flags.ExperimentStatus) error {
	return nil
}

func (m *mockExperimentRepo) CreateExperimentVariant(ctx context.Context, variant *flags.ExperimentVariant) error {
	return nil
}

type mockOutboxRepo struct{}

func (m *mockOutboxRepo) AddEvent(ctx context.Context, event *flags.OutboxEvent) error {
	fmt.Printf("Mock: Adding outbox event: %s\n", event.EventType)
	return nil
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, limit int) ([]*flags.OutboxEvent, error) {
	return nil, nil
}

func (m *mockOutboxRepo) MarkPublished(ctx context.Context, eventIDs []uuid.UUID) error {
	return nil
}