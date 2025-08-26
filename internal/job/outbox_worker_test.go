package job

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/latoulicious/RoutePilot/internal/adapters/kafka"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockOutboxRepository is a mock implementation of OutboxRepository
type MockOutboxRepository struct {
	mock.Mock
}

func (m *MockOutboxRepository) AddEvent(ctx context.Context, event *flags.OutboxEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockOutboxRepository) ClaimBatch(ctx context.Context, limit int) ([]*flags.OutboxEvent, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*flags.OutboxEvent), args.Error(1)
}

func (m *MockOutboxRepository) MarkPublished(ctx context.Context, eventIDs []uuid.UUID) error {
	args := m.Called(ctx, eventIDs)
	return args.Error(0)
}

func TestOutboxWorker_ProcessBatch_Success(t *testing.T) {
	// Setup
	mockRepo := new(MockOutboxRepository)
	publisher := kafka.NewMockPublisher()
	config := DefaultOutboxWorkerConfig()
	config.ProcessTimeout = 5 * time.Second

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Create test events
	events := []*flags.OutboxEvent{
		{
			ID:       uuid.New(),
			TenantID: uuid.New(),
			Topic:    "flag.exposures",
			Payload:  json.RawMessage(`{"flag_key":"test1","enabled":true}`),
		},
		{
			ID:       uuid.New(),
			TenantID: uuid.New(),
			Topic:    "experiment.conversions",
			Key:      stringPtr("conversion_key"),
			Payload:  json.RawMessage(`{"experiment_key":"exp1","variant":"control"}`),
		},
	}

	eventIDs := []uuid.UUID{events[0].ID, events[1].ID}

	// Setup expectations
	mockRepo.On("ClaimBatch", mock.Anything, config.BatchSize).Return(events, nil)
	mockRepo.On("MarkPublished", mock.Anything, eventIDs).Return(nil)

	// Execute
	ctx := context.Background()
	err := worker.processBatch(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, publisher.PublishedEvents, 2)
	assert.Equal(t, events[0].ID, publisher.PublishedEvents[0].ID)
	assert.Equal(t, events[1].ID, publisher.PublishedEvents[1].ID)

	mockRepo.AssertExpectations(t)
}

func TestOutboxWorker_ProcessBatch_EmptyBatch(t *testing.T) {
	// Setup
	mockRepo := new(MockOutboxRepository)
	publisher := kafka.NewMockPublisher()
	config := DefaultOutboxWorkerConfig()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Setup expectations - return empty batch
	mockRepo.On("ClaimBatch", mock.Anything, config.BatchSize).Return([]*flags.OutboxEvent{}, nil)

	// Execute
	ctx := context.Background()
	err := worker.processBatch(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, publisher.PublishedEvents)

	mockRepo.AssertExpectations(t)
}

func TestOutboxWorker_ProcessBatch_ClaimError(t *testing.T) {
	// Setup
	mockRepo := new(MockOutboxRepository)
	publisher := kafka.NewMockPublisher()
	config := DefaultOutboxWorkerConfig()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Setup expectations - return error
	expectedError := errors.New("database connection failed")
	mockRepo.On("ClaimBatch", mock.Anything, config.BatchSize).Return([]*flags.OutboxEvent{}, expectedError)

	// Execute
	ctx := context.Background()
	err := worker.processBatch(ctx)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to claim outbox batch")
	assert.Empty(t, publisher.PublishedEvents)

	mockRepo.AssertExpectations(t)
}

func TestOutboxWorker_ProcessBatch_PublishError(t *testing.T) {
	// Setup
	mockRepo := new(MockOutboxRepository)
	publisher := kafka.NewMockPublisher()
	publisher.ShouldFail = true
	config := DefaultOutboxWorkerConfig()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Create test events
	events := []*flags.OutboxEvent{
		{
			ID:       uuid.New(),
			TenantID: uuid.New(),
			Topic:    "flag.exposures",
			Payload:  json.RawMessage(`{"flag_key":"test","enabled":true}`),
		},
	}

	// Setup expectations
	mockRepo.On("ClaimBatch", mock.Anything, config.BatchSize).Return(events, nil)
	// Note: MarkPublished should NOT be called when publish fails

	// Execute
	ctx := context.Background()
	err := worker.processBatch(ctx)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to publish events")
	assert.Empty(t, publisher.PublishedEvents) // Mock publisher doesn't add events on failure

	mockRepo.AssertExpectations(t)
}

func TestOutboxWorker_ProcessBatch_MarkPublishedError(t *testing.T) {
	// Setup
	mockRepo := new(MockOutboxRepository)
	publisher := kafka.NewMockPublisher()
	config := DefaultOutboxWorkerConfig()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Create test events
	events := []*flags.OutboxEvent{
		{
			ID:       uuid.New(),
			TenantID: uuid.New(),
			Topic:    "flag.exposures",
			Payload:  json.RawMessage(`{"flag_key":"test","enabled":true}`),
		},
	}

	eventIDs := []uuid.UUID{events[0].ID}

	// Setup expectations
	mockRepo.On("ClaimBatch", mock.Anything, config.BatchSize).Return(events, nil)
	expectedError := errors.New("database update failed")
	mockRepo.On("MarkPublished", mock.Anything, eventIDs).Return(expectedError)

	// Execute
	ctx := context.Background()
	err := worker.processBatch(ctx)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to mark events as published")
	assert.Len(t, publisher.PublishedEvents, 1) // Events were published successfully

	mockRepo.AssertExpectations(t)
}

func TestOutboxWorker_StartStop(t *testing.T) {
	// Setup
	mockRepo := new(MockOutboxRepository)
	publisher := kafka.NewMockPublisher()
	config := DefaultOutboxWorkerConfig()
	config.PollInterval = 100 * time.Millisecond // Fast polling for test
	config.ShutdownTimeout = 1 * time.Second

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Setup expectations - return empty batches
	mockRepo.On("ClaimBatch", mock.Anything, config.BatchSize).Return([]*flags.OutboxEvent{}, nil).Maybe()

	// Start worker
	ctx := context.Background()
	err := worker.Start(ctx)
	require.NoError(t, err)

	// Let it run briefly
	time.Sleep(200 * time.Millisecond)

	// Stop worker
	err = worker.Stop()
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestOutboxWorker_Health(t *testing.T) {
	// Setup
	mockRepo := new(MockOutboxRepository)
	publisher := kafka.NewMockPublisher()
	config := DefaultOutboxWorkerConfig()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Execute
	health := worker.Health()

	// Assert
	assert.Equal(t, "running", health["status"])
	assert.Equal(t, config.BatchSize, health["batch_size"])
	assert.Equal(t, config.PollInterval.String(), health["poll_interval"])
}

func TestDefaultOutboxWorkerConfig(t *testing.T) {
	config := DefaultOutboxWorkerConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 100, config.BatchSize)
	assert.Equal(t, 5*time.Second, config.PollInterval)
	assert.Equal(t, 30*time.Second, config.ProcessTimeout)
	assert.Equal(t, 30*time.Second, config.ShutdownTimeout)
}

func stringPtr(s string) *string {
	return &s
}
