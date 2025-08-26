package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockPublisher(t *testing.T) {
	tests := []struct {
		name        string
		events      []*flags.OutboxEvent
		shouldFail  bool
		expectError bool
	}{
		{
			name:        "empty events",
			events:      []*flags.OutboxEvent{},
			shouldFail:  false,
			expectError: false,
		},
		{
			name: "single event",
			events: []*flags.OutboxEvent{
				{
					ID:       uuid.New(),
					TenantID: uuid.New(),
					Topic:    "flag.exposures",
					Payload:  json.RawMessage(`{"flag_key":"test","enabled":true}`),
				},
			},
			shouldFail:  false,
			expectError: false,
		},
		{
			name: "multiple events",
			events: []*flags.OutboxEvent{
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
			},
			shouldFail:  false,
			expectError: false,
		},
		{
			name: "publisher failure",
			events: []*flags.OutboxEvent{
				{
					ID:       uuid.New(),
					TenantID: uuid.New(),
					Topic:    "flag.exposures",
					Payload:  json.RawMessage(`{"flag_key":"test","enabled":true}`),
				},
			},
			shouldFail:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := NewMockPublisher()
			publisher.ShouldFail = tt.shouldFail

			ctx := context.Background()
			err := publisher.PublishEvents(ctx, tt.events)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, publisher.PublishedEvents)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.events), len(publisher.PublishedEvents))

				for i, event := range tt.events {
					assert.Equal(t, event.ID, publisher.PublishedEvents[i].ID)
					assert.Equal(t, event.TenantID, publisher.PublishedEvents[i].TenantID)
					assert.Equal(t, event.Topic, publisher.PublishedEvents[i].Topic)
					assert.Equal(t, event.Payload, publisher.PublishedEvents[i].Payload)
				}
			}

			assert.NoError(t, publisher.Close())
		})
	}
}

func TestMockPublisherReset(t *testing.T) {
	publisher := NewMockPublisher()

	// Add some events
	events := []*flags.OutboxEvent{
		{
			ID:       uuid.New(),
			TenantID: uuid.New(),
			Topic:    "test.topic",
			Payload:  json.RawMessage(`{"test":true}`),
		},
	}

	err := publisher.PublishEvents(context.Background(), events)
	require.NoError(t, err)
	assert.Len(t, publisher.PublishedEvents, 1)

	// Reset and verify
	publisher.Reset()
	assert.Empty(t, publisher.PublishedEvents)
	assert.False(t, publisher.ShouldFail)
	assert.Nil(t, publisher.FailureError)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, []string{"localhost:9092"}, config.Brokers)
	assert.Equal(t, 3, config.RetryMax)
	assert.Equal(t, 100*time.Millisecond, config.RetryBackoff)
	assert.Equal(t, 10*time.Second, config.FlushTimeout)
}

func TestSaramaPublisherCreation(t *testing.T) {
	// Skip if no Kafka available
	if os.Getenv("KAFKA_BROKERS") == "" {
		t.Skip("Skipping Kafka integration test - KAFKA_BROKERS not set")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	config := DefaultConfig()
	config.Brokers = []string{os.Getenv("KAFKA_BROKERS")}

	publisher, err := NewSaramaPublisher(config, logger)
	if err != nil {
		t.Skipf("Skipping Kafka integration test - failed to create publisher: %v", err)
	}
	defer publisher.Close()

	assert.NotNil(t, publisher)
}

func stringPtr(s string) *string {
	return &s
}
