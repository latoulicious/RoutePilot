package job

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/latoulicious/RoutePilot/internal/adapters/kafka"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockOutboxRepositoryForIntegration is a simple mock for integration testing
type MockOutboxRepositoryForIntegration struct {
	events []*flags.OutboxEvent
}

func NewMockOutboxRepositoryForIntegration() *MockOutboxRepositoryForIntegration {
	return &MockOutboxRepositoryForIntegration{
		events: make([]*flags.OutboxEvent, 0),
	}
}

func (m *MockOutboxRepositoryForIntegration) AddEvent(ctx context.Context, event *flags.OutboxEvent) error {
	event.ID = uuid.New()
	event.CreatedAt = time.Now()
	m.events = append(m.events, event)
	return nil
}

func (m *MockOutboxRepositoryForIntegration) ClaimBatch(ctx context.Context, limit int) ([]*flags.OutboxEvent, error) {
	// Return unpublished events
	unpublished := make([]*flags.OutboxEvent, 0)
	for _, event := range m.events {
		if event.PublishedAt == nil {
			unpublished = append(unpublished, event)
		}
		if len(unpublished) >= limit {
			break
		}
	}
	return unpublished, nil
}

func (m *MockOutboxRepositoryForIntegration) MarkPublished(ctx context.Context, eventIDs []uuid.UUID) error {
	now := time.Now()
	for _, event := range m.events {
		for _, id := range eventIDs {
			if event.ID == id {
				event.PublishedAt = &now
				break
			}
		}
	}
	return nil
}

func TestOutboxWorkerIntegration(t *testing.T) {
	// Setup
	mockRepo := NewMockOutboxRepositoryForIntegration()
	publisher := kafka.NewMockPublisher()

	config := &OutboxWorkerConfig{
		BatchSize:       10,
		PollInterval:    100 * time.Millisecond, // Fast polling for test
		ProcessTimeout:  5 * time.Second,
		ShutdownTimeout: 2 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Add some test events
	testEvents := []*flags.OutboxEvent{
		{
			TenantID: uuid.New(),
			Topic:    "flag.exposures",
			Payload:  json.RawMessage(`{"flag_key":"test1","enabled":true}`),
		},
		{
			TenantID: uuid.New(),
			Topic:    "experiment.conversions",
			Key:      stringPtr("conversion_key"),
			Payload:  json.RawMessage(`{"experiment_key":"exp1","variant":"control"}`),
		},
	}

	ctx := context.Background()
	for _, event := range testEvents {
		err := mockRepo.AddEvent(ctx, event)
		require.NoError(t, err)
	}

	// Start worker
	err := worker.Start(ctx)
	require.NoError(t, err)

	// Wait for events to be processed
	time.Sleep(300 * time.Millisecond)

	// Stop worker
	err = worker.Stop()
	require.NoError(t, err)

	// Verify events were published
	assert.Len(t, publisher.PublishedEvents, 2)

	// Verify events were marked as published
	for _, event := range mockRepo.events {
		assert.NotNil(t, event.PublishedAt, "Event should be marked as published")
	}

	// Verify health status
	health := worker.Health()
	assert.Equal(t, "running", health["status"])
	assert.Equal(t, config.BatchSize, health["batch_size"])
}

func TestOutboxWorkerIntegration_NoEvents(t *testing.T) {
	// Setup with no events
	mockRepo := NewMockOutboxRepositoryForIntegration()
	publisher := kafka.NewMockPublisher()

	config := &OutboxWorkerConfig{
		BatchSize:       10,
		PollInterval:    50 * time.Millisecond, // Fast polling for test
		ProcessTimeout:  1 * time.Second,
		ShutdownTimeout: 1 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	worker := NewOutboxWorker(mockRepo, publisher, config, logger)

	// Start worker
	ctx := context.Background()
	err := worker.Start(ctx)
	require.NoError(t, err)

	// Wait briefly
	time.Sleep(100 * time.Millisecond)

	// Stop worker
	err = worker.Stop()
	require.NoError(t, err)

	// Verify no events were published
	assert.Empty(t, publisher.PublishedEvents)
}
