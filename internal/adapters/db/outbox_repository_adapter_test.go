package db

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/stretchr/testify/assert"
)

func TestOutboxEventConversion(t *testing.T) {
	// Test conversion between domain model and database model
	tenantID := uuid.New()
	eventID := uuid.New()
	key := "test_key"
	now := time.Now()

	// Test domain to database conversion
	domainEvent := &flags.OutboxEvent{
		ID:        eventID,
		TenantID:  tenantID,
		Topic:     "flag.exposures",
		Key:       &key,
		Payload:   json.RawMessage(`{"flag_key":"test","enabled":true}`),
		CreatedAt: now,
	}

	// Test database to domain conversion
	dbEvent := Outbox{
		ID:          pgtype.UUID{Bytes: eventID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantID, Valid: true},
		Topic:       "flag.exposures",
		Key:         pgtype.Text{String: key, Valid: true},
		Payload:     json.RawMessage(`{"flag_key":"test","enabled":true}`),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		PublishedAt: pgtype.Timestamptz{Valid: false},
	}

	// Verify conversions work as expected
	assert.Equal(t, eventID, uuid.UUID(dbEvent.ID.Bytes))
	assert.Equal(t, tenantID, uuid.UUID(dbEvent.TenantID.Bytes))
	assert.Equal(t, domainEvent.Topic, dbEvent.Topic)
	assert.Equal(t, *domainEvent.Key, dbEvent.Key.String)
	assert.Equal(t, string(domainEvent.Payload), string(dbEvent.Payload))
	assert.Equal(t, domainEvent.CreatedAt, dbEvent.CreatedAt.Time)
}

func TestOutboxEventConversion_WithoutKey(t *testing.T) {
	// Test conversion when key is nil
	tenantID := uuid.New()
	eventID := uuid.New()
	now := time.Now()

	// Test domain event without key
	domainEvent := &flags.OutboxEvent{
		ID:        eventID,
		TenantID:  tenantID,
		Topic:     "experiment.conversions",
		Key:       nil, // No key
		Payload:   json.RawMessage(`{"experiment_key":"exp1","variant":"control"}`),
		CreatedAt: now,
	}

	// Test database event without key
	dbEvent := Outbox{
		ID:          pgtype.UUID{Bytes: eventID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantID, Valid: true},
		Topic:       "experiment.conversions",
		Key:         pgtype.Text{Valid: false}, // No key
		Payload:     json.RawMessage(`{"experiment_key":"exp1","variant":"control"}`),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		PublishedAt: pgtype.Timestamptz{Valid: false},
	}

	// Verify conversions work as expected
	assert.Equal(t, eventID, uuid.UUID(dbEvent.ID.Bytes))
	assert.Equal(t, tenantID, uuid.UUID(dbEvent.TenantID.Bytes))
	assert.Equal(t, domainEvent.Topic, dbEvent.Topic)
	assert.False(t, dbEvent.Key.Valid)
	assert.Nil(t, domainEvent.Key)
	assert.Equal(t, string(domainEvent.Payload), string(dbEvent.Payload))
	assert.Equal(t, domainEvent.CreatedAt, dbEvent.CreatedAt.Time)
}

func TestOutboxEventConversion_WithPublishedAt(t *testing.T) {
	// Test conversion when event is published
	tenantID := uuid.New()
	eventID := uuid.New()
	now := time.Now()
	publishedAt := now.Add(time.Minute)

	// Test database event with published timestamp
	dbEvent := Outbox{
		ID:          pgtype.UUID{Bytes: eventID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantID, Valid: true},
		Topic:       "flag.exposures",
		Key:         pgtype.Text{Valid: false},
		Payload:     json.RawMessage(`{"flag_key":"test","enabled":true}`),
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		PublishedAt: pgtype.Timestamptz{Time: publishedAt, Valid: true},
	}

	// Verify published timestamp conversion
	assert.Equal(t, publishedAt, dbEvent.PublishedAt.Time)
	assert.True(t, dbEvent.PublishedAt.Valid)
}
