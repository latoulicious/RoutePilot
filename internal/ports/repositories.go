package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
)

// FlagRepository defines the interface for flag data access
type FlagRepository interface {
	GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*flags.Flag, error)
	GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*flags.FlagRule, error)
	CreateFlag(ctx context.Context, flag *flags.Flag) error
	UpdateFlag(ctx context.Context, flagID uuid.UUID, updates flags.FlagUpdates) error
	DeleteFlag(ctx context.Context, flagID uuid.UUID) error
}

// AssignmentRepository defines the interface for assignment data access
type AssignmentRepository interface {
	GetAssignment(ctx context.Context, flagID uuid.UUID, subjectID string) (*flags.Assignment, error)
	UpsertAssignment(ctx context.Context, assignment *flags.Assignment) error
}

// ExperimentRepository defines the interface for experiment data access
type ExperimentRepository interface {
	GetExperimentByFlagID(ctx context.Context, flagID uuid.UUID) (*flags.Experiment, error)
	GetExperimentVariants(ctx context.Context, experimentID uuid.UUID) ([]*flags.ExperimentVariant, error)
	CreateExperiment(ctx context.Context, experiment *flags.Experiment) error
	UpdateExperiment(ctx context.Context, experimentID uuid.UUID, status flags.ExperimentStatus) error
	CreateExperimentVariant(ctx context.Context, variant *flags.ExperimentVariant) error
}

// TenantRepository defines the interface for tenant data access
type TenantRepository interface {
	GetTenantByID(ctx context.Context, tenantID uuid.UUID) (*flags.Tenant, error)
	CreateTenant(ctx context.Context, tenant *flags.Tenant) error
}

// APIKeyRepository defines the interface for API key data access
type APIKeyRepository interface {
	GetAPIKeyByKeyID(ctx context.Context, keyID string) (*flags.APIKey, error)
	CreateAPIKey(ctx context.Context, apiKey *flags.APIKey) error
	DeleteAPIKey(ctx context.Context, keyID string) error
}

// OutboxRepository defines the interface for outbox event data access
type OutboxRepository interface {
	AddEvent(ctx context.Context, event *flags.OutboxEvent) error
	ClaimBatch(ctx context.Context, limit int) ([]*flags.OutboxEvent, error)
	MarkPublished(ctx context.Context, eventIDs []uuid.UUID) error
}