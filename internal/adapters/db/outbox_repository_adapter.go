package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
)

// OutboxRepositoryAdapter implements the OutboxRepository interface
type OutboxRepositoryAdapter struct {
	repo *Repository
}

// NewOutboxRepositoryAdapter creates a new outbox repository adapter
func NewOutboxRepositoryAdapter(repo *Repository) *OutboxRepositoryAdapter {
	return &OutboxRepositoryAdapter{repo: repo}
}

// AddEvent adds a new event to the outbox
func (r *OutboxRepositoryAdapter) AddEvent(ctx context.Context, event *flags.OutboxEvent) error {
	var key pgtype.Text
	if event.Key != nil {
		key = pgtype.Text{String: *event.Key, Valid: true}
	}

	params := AddOutboxParams{
		TenantID: pgtype.UUID{Bytes: event.TenantID, Valid: true},
		Topic:    event.Topic,
		Key:      key,
		Payload:  event.Payload,
	}

	outboxEvent, err := r.repo.AddOutbox(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to add outbox event: %w", err)
	}

	// Update the event with the generated ID and timestamps
	event.ID = uuid.UUID(outboxEvent.ID.Bytes)
	event.CreatedAt = outboxEvent.CreatedAt.Time
	if outboxEvent.PublishedAt.Valid {
		publishedAt := outboxEvent.PublishedAt.Time
		event.PublishedAt = &publishedAt
	}

	return nil
}

// ClaimBatch claims a batch of unpublished events for processing
func (r *OutboxRepositoryAdapter) ClaimBatch(ctx context.Context, limit int) ([]*flags.OutboxEvent, error) {
	outboxEvents, err := r.repo.ClaimOutboxBatch(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to claim outbox batch: %w", err)
	}

	events := make([]*flags.OutboxEvent, len(outboxEvents))
	for i, dbEvent := range outboxEvents {
		event := &flags.OutboxEvent{
			ID:        uuid.UUID(dbEvent.ID.Bytes),
			TenantID:  uuid.UUID(dbEvent.TenantID.Bytes),
			Topic:     dbEvent.Topic,
			Payload:   dbEvent.Payload,
			CreatedAt: dbEvent.CreatedAt.Time,
		}

		// Handle optional key
		if dbEvent.Key.Valid {
			key := dbEvent.Key.String
			event.Key = &key
		}

		// Handle published timestamp
		if dbEvent.PublishedAt.Valid {
			publishedAt := dbEvent.PublishedAt.Time
			event.PublishedAt = &publishedAt
		}

		events[i] = event
	}

	return events, nil
}

// MarkPublished marks events as published
func (r *OutboxRepositoryAdapter) MarkPublished(ctx context.Context, eventIDs []uuid.UUID) error {
	if len(eventIDs) == 0 {
		return nil
	}

	// Convert UUIDs to pgtype.UUID
	pgUUIDs := make([]pgtype.UUID, len(eventIDs))
	for i, id := range eventIDs {
		pgUUIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}

	err := r.repo.MarkOutboxPublished(ctx, pgUUIDs)
	if err != nil {
		return fmt.Errorf("failed to mark events as published: %w", err)
	}

	return nil
}
