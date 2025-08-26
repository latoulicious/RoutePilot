package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/IBM/sarama"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
)

// Publisher defines the interface for publishing events to Kafka
type Publisher interface {
	PublishEvents(ctx context.Context, events []*flags.OutboxEvent) error
	Close() error
}

// Config holds the configuration for Kafka publisher
type Config struct {
	Brokers      []string
	RetryMax     int
	RetryBackoff time.Duration
	FlushTimeout time.Duration
	RequiredAcks sarama.RequiredAcks
	Compression  sarama.CompressionCodec
}

// DefaultConfig returns a default Kafka configuration
func DefaultConfig() *Config {
	return &Config{
		Brokers:      []string{"localhost:9092"},
		RetryMax:     3,
		RetryBackoff: 100 * time.Millisecond,
		FlushTimeout: 10 * time.Second,
		RequiredAcks: sarama.WaitForAll,
		Compression:  sarama.CompressionSnappy,
	}
}

// SaramaPublisher implements Publisher using Sarama Kafka client
type SaramaPublisher struct {
	producer sarama.SyncProducer
	config   *Config
	logger   *slog.Logger
}

// NewSaramaPublisher creates a new Kafka publisher using Sarama
func NewSaramaPublisher(config *Config, logger *slog.Logger) (*SaramaPublisher, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.RequiredAcks = config.RequiredAcks
	saramaConfig.Producer.Retry.Max = config.RetryMax
	saramaConfig.Producer.Retry.Backoff = config.RetryBackoff
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true
	saramaConfig.Producer.Compression = config.Compression
	saramaConfig.Producer.Flush.Frequency = config.FlushTimeout
	saramaConfig.Producer.Idempotent = true
	saramaConfig.Net.MaxOpenRequests = 1

	producer, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	return &SaramaPublisher{
		producer: producer,
		config:   config,
		logger:   logger,
	}, nil
}

// PublishEvents publishes a batch of outbox events to Kafka
func (p *SaramaPublisher) PublishEvents(ctx context.Context, events []*flags.OutboxEvent) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]*sarama.ProducerMessage, 0, len(events))

	for _, event := range events {
		message := &sarama.ProducerMessage{
			Topic: event.Topic,
			Value: sarama.ByteEncoder(event.Payload),
		}

		// Set partition key if provided
		if event.Key != nil {
			message.Key = sarama.StringEncoder(*event.Key)
		}

		// Add headers for tracing and metadata
		message.Headers = []sarama.RecordHeader{
			{
				Key:   []byte("event_id"),
				Value: []byte(event.ID.String()),
			},
			{
				Key:   []byte("tenant_id"),
				Value: []byte(event.TenantID.String()),
			},
			{
				Key:   []byte("created_at"),
				Value: []byte(event.CreatedAt.Format(time.RFC3339)),
			},
		}

		messages = append(messages, message)
	}

	// Publish messages in batch
	for i, message := range messages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		partition, offset, err := p.producer.SendMessage(message)
		if err != nil {
			p.logger.Error("Failed to publish message to Kafka",
				"error", err,
				"topic", message.Topic,
				"event_id", events[i].ID,
				"tenant_id", events[i].TenantID,
			)
			return fmt.Errorf("failed to publish message to topic %s: %w", message.Topic, err)
		}

		p.logger.Debug("Successfully published message to Kafka",
			"topic", message.Topic,
			"partition", partition,
			"offset", offset,
			"event_id", events[i].ID,
			"tenant_id", events[i].TenantID,
		)
	}

	p.logger.Info("Successfully published batch to Kafka",
		"batch_size", len(events),
	)

	return nil
}

// Close closes the Kafka producer
func (p *SaramaPublisher) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}

// MockPublisher is a mock implementation for testing
type MockPublisher struct {
	PublishedEvents []*flags.OutboxEvent
	ShouldFail      bool
	FailureError    error
}

// NewMockPublisher creates a new mock publisher for testing
func NewMockPublisher() *MockPublisher {
	return &MockPublisher{
		PublishedEvents: make([]*flags.OutboxEvent, 0),
	}
}

// PublishEvents mock implementation
func (m *MockPublisher) PublishEvents(ctx context.Context, events []*flags.OutboxEvent) error {
	if m.ShouldFail {
		if m.FailureError != nil {
			return m.FailureError
		}
		return fmt.Errorf("mock publisher failure")
	}

	m.PublishedEvents = append(m.PublishedEvents, events...)
	return nil
}

// Close mock implementation
func (m *MockPublisher) Close() error {
	return nil
}

// Reset clears the published events for testing
func (m *MockPublisher) Reset() {
	m.PublishedEvents = make([]*flags.OutboxEvent, 0)
	m.ShouldFail = false
	m.FailureError = nil
}
