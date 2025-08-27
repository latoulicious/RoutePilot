package flags

import (
    "encoding/json"
    "time"

    "github.com/google/uuid"
)

// FlagType represents the type of flag value
type FlagType string

const (
    FlagTypeBoolean FlagType = "boolean"
    FlagTypeJSON    FlagType = "json"
)

// ExperimentStatus represents the status of an experiment
type ExperimentStatus string

const (
    ExperimentStatusDraft   ExperimentStatus = "draft"
    ExperimentStatusRunning ExperimentStatus = "running"
    ExperimentStatusPaused  ExperimentStatus = "paused"
    ExperimentStatusStopped ExperimentStatus = "stopped"
)

// Flag represents a feature flag
type Flag struct {
    ID          uuid.UUID `json:"id"`
    TenantID    uuid.UUID `json:"tenant_id"`
    Key         string    `json:"key"`
    Description string    `json:"description"`
    Type        FlagType  `json:"type"`
    Enabled     bool      `json:"enabled"`
    Salt        string    `json:"salt"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// FlagRule represents a rule for flag evaluation
type FlagRule struct {
    ID       uuid.UUID       `json:"id"`
    FlagID   uuid.UUID       `json:"flag_id"`
    Priority int             `json:"priority"`
    Rollout  int             `json:"rollout"` // 0-100
    Variant  json.RawMessage `json:"variant"`
}

// Assignment represents a sticky assignment for a subject
type Assignment struct {
    ID            uuid.UUID       `json:"id"`
    FlagID        uuid.UUID       `json:"flag_id"`
    SubjectID     string          `json:"subject_id"`
    Bucket        int             `json:"bucket"` // 0-9999
    ChosenVariant json.RawMessage `json:"chosen_variant"`
    AssignedAt    time.Time       `json:"assigned_at"`
}

// Experiment represents an A/B test experiment
type Experiment struct {
    ID       uuid.UUID            `json:"id"`
    TenantID uuid.UUID            `json:"tenant_id"`
    Key      string               `json:"key"`
    FlagID   uuid.UUID            `json:"flag_id"`
    Status   ExperimentStatus     `json:"status"`
    Traffic  int                  `json:"traffic"` // 0-100
    Variants []*ExperimentVariant `json:"variants"`
}

// ExperimentVariant represents a variant in an experiment
type ExperimentVariant struct {
    ID           uuid.UUID       `json:"id"`
    ExperimentID uuid.UUID       `json:"experiment_id"`
    Name         string          `json:"name"`
    Weight       int             `json:"weight"` // 0-100
    Value        json.RawMessage `json:"value"`
}

// Tenant represents a tenant in the system
type Tenant struct {
    ID        uuid.UUID `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// APIKey represents an API key for authentication
type APIKey struct {
    ID           uuid.UUID `json:"id"`
    TenantID     uuid.UUID `json:"tenant_id"`
    Name         string    `json:"name"`
    KeyID        string    `json:"key_id"`
    SecretHash   string    `json:"secret_hash"`
    EncryptedKey string    `json:"encrypted_key"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// OutboxEvent represents an event in the outbox pattern
type OutboxEvent struct {
    ID          uuid.UUID       `json:"id"`
    TenantID    uuid.UUID       `json:"tenant_id"`
    Topic       string          `json:"topic"`
    Key         *string         `json:"key,omitempty"`
    Payload     json.RawMessage `json:"payload"`
    PublishedAt *time.Time      `json:"published_at"`
    CreatedAt   time.Time       `json:"created_at"`
}

