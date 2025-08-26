package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/latoulicious/RoutePilot/internal/adapters/kafka"
	"github.com/latoulicious/RoutePilot/internal/ports"
)

// OutboxWorkerConfig holds configuration for the outbox worker
type OutboxWorkerConfig struct {
	BatchSize       int
	PollInterval    time.Duration
	ProcessTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// DefaultOutboxWorkerConfig returns default configuration
func DefaultOutboxWorkerConfig() *OutboxWorkerConfig {
	return &OutboxWorkerConfig{
		BatchSize:       100,
		PollInterval:    5 * time.Second,
		ProcessTimeout:  30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// OutboxWorker processes outbox events and publishes them to Kafka
type OutboxWorker struct {
	outboxRepo ports.OutboxRepository
	publisher  kafka.Publisher
	config     *OutboxWorkerConfig
	logger     *slog.Logger
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// NewOutboxWorker creates a new outbox worker
func NewOutboxWorker(
	outboxRepo ports.OutboxRepository,
	publisher kafka.Publisher,
	config *OutboxWorkerConfig,
	logger *slog.Logger,
) *OutboxWorker {
	return &OutboxWorker{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		config:     config,
		logger:     logger,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Start begins the outbox worker processing loop
func (w *OutboxWorker) Start(ctx context.Context) error {
	w.logger.Info("Starting outbox worker",
		"batch_size", w.config.BatchSize,
		"poll_interval", w.config.PollInterval,
	)

	go w.processLoop(ctx)
	return nil
}

// Stop gracefully stops the outbox worker
func (w *OutboxWorker) Stop() error {
	w.logger.Info("Stopping outbox worker")

	close(w.stopCh)

	// Wait for processing to complete with timeout
	select {
	case <-w.doneCh:
		w.logger.Info("Outbox worker stopped gracefully")
	case <-time.After(w.config.ShutdownTimeout):
		w.logger.Warn("Outbox worker shutdown timeout exceeded")
	}

	return w.publisher.Close()
}

// processLoop is the main processing loop
func (w *OutboxWorker) processLoop(ctx context.Context) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Context cancelled, stopping outbox worker")
			return
		case <-w.stopCh:
			w.logger.Info("Stop signal received, stopping outbox worker")
			return
		case <-ticker.C:
			if err := w.processBatch(ctx); err != nil {
				w.logger.Error("Failed to process outbox batch", "error", err)
			}
		}
	}
}

// processBatch processes a single batch of outbox events
func (w *OutboxWorker) processBatch(ctx context.Context) error {
	// Create a timeout context for this batch
	batchCtx, cancel := context.WithTimeout(ctx, w.config.ProcessTimeout)
	defer cancel()

	// Claim a batch of events
	events, err := w.outboxRepo.ClaimBatch(batchCtx, w.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to claim outbox batch: %w", err)
	}

	if len(events) == 0 {
		w.logger.Debug("No outbox events to process")
		return nil
	}

	w.logger.Info("Processing outbox batch", "batch_size", len(events))

	// Publish events to Kafka
	if err := w.publisher.PublishEvents(batchCtx, events); err != nil {
		w.logger.Error("Failed to publish events to Kafka",
			"error", err,
			"batch_size", len(events),
		)
		// Don't mark as published on failure - events will be retried
		return fmt.Errorf("failed to publish events: %w", err)
	}

	// Mark events as published
	eventIDs := make([]uuid.UUID, len(events))
	for i, event := range events {
		eventIDs[i] = event.ID
	}

	if err := w.outboxRepo.MarkPublished(batchCtx, eventIDs); err != nil {
		w.logger.Error("Failed to mark events as published",
			"error", err,
			"batch_size", len(events),
		)
		// This is a critical error - events were published but not marked
		// They will be published again on next run (at-least-once delivery)
		return fmt.Errorf("failed to mark events as published: %w", err)
	}

	w.logger.Info("Successfully processed outbox batch",
		"batch_size", len(events),
		"published_count", len(eventIDs),
	)

	return nil
}

// Health returns the health status of the worker
func (w *OutboxWorker) Health() map[string]interface{} {
	return map[string]interface{}{
		"status":        "running",
		"batch_size":    w.config.BatchSize,
		"poll_interval": w.config.PollInterval.String(),
	}
}
