package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
    "github.com/latoulicious/RoutePilot/internal/domain/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
)

// ExperimentRepositoryAdapter adapts the database repository for experiment operations
type ExperimentRepositoryAdapter struct {
	queries *Queries
}

// Ensure ExperimentRepositoryAdapter implements the interface
var _ ports.ExperimentRepository = (*ExperimentRepositoryAdapter)(nil)

// NewExperimentRepositoryAdapter creates a new experiment repository adapter
func NewExperimentRepositoryAdapter(queries *Queries) *ExperimentRepositoryAdapter {
	return &ExperimentRepositoryAdapter{
		queries: queries,
	}
}

// GetExperimentByFlagID retrieves an experiment by flag ID
func (r *ExperimentRepositoryAdapter) GetExperimentByFlagID(ctx context.Context, flagID uuid.UUID) (*flags.Experiment, error) {
	var pgFlagID pgtype.UUID
	if err := pgFlagID.Scan(flagID); err != nil {
		return nil, fmt.Errorf("invalid flag ID: %v", err)
	}

	dbExperiment, err := r.queries.GetExperimentByFlagID(ctx, pgFlagID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("experiment not found")
		}
		return nil, fmt.Errorf("failed to get experiment: %v", err)
	}

	experiment, err := r.convertDBExperimentToCore(dbExperiment)
	if err != nil {
		return nil, err
	}

	// Get variants for this experiment
	variants, err := r.GetExperimentVariants(ctx, experiment.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get experiment variants: %v", err)
	}
	experiment.Variants = variants

	return experiment, nil
}

// GetExperimentVariants retrieves all variants for an experiment
func (r *ExperimentRepositoryAdapter) GetExperimentVariants(ctx context.Context, experimentID uuid.UUID) ([]*flags.ExperimentVariant, error) {
	var pgExperimentID pgtype.UUID
	if err := pgExperimentID.Scan(experimentID); err != nil {
		return nil, fmt.Errorf("invalid experiment ID: %v", err)
	}

	dbVariants, err := r.queries.GetVariantsByExperiment(ctx, pgExperimentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get experiment variants: %v", err)
	}

	variants := make([]*flags.ExperimentVariant, len(dbVariants))
	for i, dbVariant := range dbVariants {
		variant, err := r.convertDBVariantToCore(dbVariant)
		if err != nil {
			return nil, fmt.Errorf("failed to convert variant %d: %v", i, err)
		}
		variants[i] = variant
	}

	return variants, nil
}

// CreateExperiment creates a new experiment
func (r *ExperimentRepositoryAdapter) CreateExperiment(ctx context.Context, experiment *flags.Experiment) error {
	var pgTenantID pgtype.UUID
	if err := pgTenantID.Scan(experiment.TenantID); err != nil {
		return fmt.Errorf("invalid tenant ID: %v", err)
	}

	var pgFlagID pgtype.UUID
	if err := pgFlagID.Scan(experiment.FlagID); err != nil {
		return fmt.Errorf("invalid flag ID: %v", err)
	}

	dbExperiment, err := r.queries.CreateExperiment(ctx, CreateExperimentParams{
		TenantID: pgTenantID,
		Key:      experiment.Key,
		FlagID:   pgFlagID,
		Traffic:  int32(experiment.Traffic),
	})
	if err != nil {
		return fmt.Errorf("failed to create experiment: %v", err)
	}

	// Update the experiment with the generated ID and timestamps
	experimentUUID, err := uuid.FromBytes(dbExperiment.ID.Bytes[:])
	if err != nil {
		return fmt.Errorf("failed to convert experiment ID: %v", err)
	}
	experiment.ID = experimentUUID

	return nil
}

// UpdateExperiment updates an experiment's status
func (r *ExperimentRepositoryAdapter) UpdateExperiment(ctx context.Context, experimentID uuid.UUID, status flags.ExperimentStatus) error {
	// First get the experiment to get tenant ID and key
	experiment, err := r.GetExperimentByID(ctx, experimentID)
	if err != nil {
		return fmt.Errorf("failed to get experiment for update: %v", err)
	}

	var pgTenantID pgtype.UUID
	if err := pgTenantID.Scan(experiment.TenantID); err != nil {
		return fmt.Errorf("invalid tenant ID: %v", err)
	}

	var pgStatus NullExperimentStatus
	pgStatus.ExperimentStatus = ExperimentStatus(status)
	pgStatus.Valid = true

	_, err = r.queries.UpdateExperimentStatus(ctx, UpdateExperimentStatusParams{
		TenantID: pgTenantID,
		Key:      experiment.Key,
		Status:   pgStatus,
	})
	if err != nil {
		return fmt.Errorf("failed to update experiment: %v", err)
	}

	return nil
}

// CreateExperimentVariant creates a new experiment variant
func (r *ExperimentRepositoryAdapter) CreateExperimentVariant(ctx context.Context, variant *flags.ExperimentVariant) error {
	var pgExperimentID pgtype.UUID
	if err := pgExperimentID.Scan(variant.ExperimentID); err != nil {
		return fmt.Errorf("invalid experiment ID: %v", err)
	}

	config, err := json.Marshal(variant.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal variant value: %v", err)
	}

	dbVariant, err := r.queries.CreateExperimentVariant(ctx, CreateExperimentVariantParams{
		ExperimentID: pgExperimentID,
		Name:         variant.Name,
		Weight:       int32(variant.Weight),
		Config:       config,
	})
	if err != nil {
		return fmt.Errorf("failed to create experiment variant: %v", err)
	}

	// Update the variant with the generated ID
	variantUUID, err := uuid.FromBytes(dbVariant.ID.Bytes[:])
	if err != nil {
		return fmt.Errorf("failed to convert variant ID: %v", err)
	}
	variant.ID = variantUUID

	return nil
}

// GetExperimentByTenantKey retrieves an experiment by tenant ID and key
func (r *ExperimentRepositoryAdapter) GetExperimentByTenantKey(ctx context.Context, tenantID uuid.UUID, key string) (*flags.Experiment, error) {
	var pgTenantID pgtype.UUID
	if err := pgTenantID.Scan(tenantID); err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %v", err)
	}

	dbExperiment, err := r.queries.GetExperimentByTenantKey(ctx, GetExperimentByTenantKeyParams{
		TenantID: pgTenantID,
		Key:      key,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("experiment not found")
		}
		return nil, fmt.Errorf("failed to get experiment: %v", err)
	}

	experiment, err := r.convertDBExperimentToCore(dbExperiment)
	if err != nil {
		return nil, err
	}

	// Get variants for this experiment
	variants, err := r.GetExperimentVariants(ctx, experiment.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get experiment variants: %v", err)
	}
	experiment.Variants = variants

	return experiment, nil
}

// GetExperimentByID is a helper method to get experiment by ID
func (r *ExperimentRepositoryAdapter) GetExperimentByID(ctx context.Context, experimentID uuid.UUID) (*flags.Experiment, error) {
	var pgExperimentID pgtype.UUID
	if err := pgExperimentID.Scan(experimentID); err != nil {
		return nil, fmt.Errorf("invalid experiment ID: %v", err)
	}

	dbExperiment, err := r.queries.GetExperimentByID(ctx, pgExperimentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("experiment not found")
		}
		return nil, fmt.Errorf("failed to get experiment: %v", err)
	}

	return r.convertDBExperimentToCore(dbExperiment)
}

// convertDBExperimentToCore converts a database experiment to core domain model
func (r *ExperimentRepositoryAdapter) convertDBExperimentToCore(dbExperiment Experiment) (*flags.Experiment, error) {
	experimentUUID, err := uuid.FromBytes(dbExperiment.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert experiment ID: %v", err)
	}

	tenantUUID, err := uuid.FromBytes(dbExperiment.TenantID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert tenant ID: %v", err)
	}

	flagUUID, err := uuid.FromBytes(dbExperiment.FlagID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert flag ID: %v", err)
	}

	experiment := &flags.Experiment{
		ID:       experimentUUID,
		TenantID: tenantUUID,
		Key:      dbExperiment.Key,
		FlagID:   flagUUID,
		Traffic:  int(dbExperiment.Traffic),
	}

	if dbExperiment.Status.Valid {
		experiment.Status = flags.ExperimentStatus(dbExperiment.Status.ExperimentStatus)
	} else {
		experiment.Status = flags.ExperimentStatusDraft
	}

	return experiment, nil
}

// convertDBVariantToCore converts a database experiment variant to core domain model
func (r *ExperimentRepositoryAdapter) convertDBVariantToCore(dbVariant ExperimentVariant) (*flags.ExperimentVariant, error) {
	variantUUID, err := uuid.FromBytes(dbVariant.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert variant ID: %v", err)
	}

	experimentUUID, err := uuid.FromBytes(dbVariant.ExperimentID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert experiment ID: %v", err)
	}

	variant := &flags.ExperimentVariant{
		ID:           variantUUID,
		ExperimentID: experimentUUID,
		Name:         dbVariant.Name,
		Weight:       int(dbVariant.Weight),
		Value:        json.RawMessage(dbVariant.Config),
	}

	return variant, nil
}
