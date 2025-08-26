package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
)

// FlagRepositoryAdapter adapts the database repository for flag operations
type FlagRepositoryAdapter struct {
	queries *Queries
}

// Ensure FlagRepositoryAdapter implements both interfaces
var _ ports.FlagRepository = (*FlagRepositoryAdapter)(nil)

// NewFlagRepositoryAdapter creates a new flag repository adapter
func NewFlagRepositoryAdapter(queries *Queries) *FlagRepositoryAdapter {
	return &FlagRepositoryAdapter{
		queries: queries,
	}
}

// GetFlagByKey retrieves a flag by tenant ID and key
func (r *FlagRepositoryAdapter) GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*flags.Flag, error) {
	var pgTenantID pgtype.UUID
	if err := pgTenantID.Scan(tenantID); err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %v", err)
	}

	dbFlag, err := r.queries.GetFlagByTenantKey(ctx, GetFlagByTenantKeyParams{
		TenantID: pgTenantID,
		Key:      key,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("flag not found")
		}
		return nil, fmt.Errorf("failed to get flag: %v", err)
	}

	return r.convertDBFlagToCore(dbFlag)
}

// GetFlagRules retrieves all rules for a flag
func (r *FlagRepositoryAdapter) GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*flags.FlagRule, error) {
	var pgFlagID pgtype.UUID
	if err := pgFlagID.Scan(flagID); err != nil {
		return nil, fmt.Errorf("invalid flag ID: %v", err)
	}

	dbRules, err := r.queries.GetRulesByFlag(ctx, pgFlagID)
	if err != nil {
		return nil, fmt.Errorf("failed to get flag rules: %v", err)
	}

	rules := make([]*flags.FlagRule, len(dbRules))
	for i, dbRule := range dbRules {
		rule, err := r.convertDBRuleToCore(dbRule)
		if err != nil {
			return nil, fmt.Errorf("failed to convert rule %d: %v", i, err)
		}
		rules[i] = rule
	}

	return rules, nil
}

// CreateFlag creates a new flag
func (r *FlagRepositoryAdapter) CreateFlag(ctx context.Context, flag *flags.Flag) error {
	var pgTenantID pgtype.UUID
	if err := pgTenantID.Scan(flag.TenantID); err != nil {
		return fmt.Errorf("invalid tenant ID: %v", err)
	}

	var pgDescription pgtype.Text
	if flag.Description != "" {
		pgDescription.String = flag.Description
		pgDescription.Valid = true
	}

	var pgEnabled pgtype.Bool
	pgEnabled.Bool = flag.Enabled
	pgEnabled.Valid = true

	dbFlag, err := r.queries.CreateFlag(ctx, CreateFlagParams{
		TenantID:    pgTenantID,
		Key:         flag.Key,
		Description: pgDescription,
		Type:        FlagType(flag.Type),
		Enabled:     pgEnabled,
		Salt:        flag.Salt,
	})
	if err != nil {
		return fmt.Errorf("failed to create flag: %v", err)
	}

	// Update the flag with the generated ID and timestamps
	flagUUID, err := uuid.FromBytes(dbFlag.ID.Bytes[:])
	if err != nil {
		return fmt.Errorf("failed to convert flag ID: %v", err)
	}
	flag.ID = flagUUID

	if dbFlag.CreatedAt.Valid {
		flag.CreatedAt = dbFlag.CreatedAt.Time
	}
	if dbFlag.UpdatedAt.Valid {
		flag.UpdatedAt = dbFlag.UpdatedAt.Time
	}

	return nil
}

// UpdateFlag updates an existing flag
func (r *FlagRepositoryAdapter) UpdateFlag(ctx context.Context, flagID uuid.UUID, updates flags.FlagUpdates) error {
	// First get the flag to get tenant ID and key
	flag, err := r.GetFlagByID(ctx, flagID)
	if err != nil {
		return fmt.Errorf("failed to get flag for update: %v", err)
	}

	var pgTenantID pgtype.UUID
	if err := pgTenantID.Scan(flag.TenantID); err != nil {
		return fmt.Errorf("invalid tenant ID: %v", err)
	}

	var pgEnabled pgtype.Bool
	if updates.Enabled != nil {
		pgEnabled.Bool = *updates.Enabled
		pgEnabled.Valid = true
	}

	var pgDescription pgtype.Text
	if updates.Description != nil {
		pgDescription.String = *updates.Description
		pgDescription.Valid = true
	}

	salt := ""
	if updates.Salt != nil {
		salt = *updates.Salt
	}

	_, err = r.queries.UpdateFlag(ctx, UpdateFlagParams{
		TenantID:    pgTenantID,
		Key:         flag.Key,
		Enabled:     pgEnabled,
		Salt:        salt,
		Description: pgDescription,
	})
	if err != nil {
		return fmt.Errorf("failed to update flag: %v", err)
	}

	return nil
}

// DeleteFlag deletes a flag (not implemented in SQL yet, but interface requires it)
func (r *FlagRepositoryAdapter) DeleteFlag(ctx context.Context, flagID uuid.UUID) error {
	return fmt.Errorf("delete flag not implemented")
}

// GetFlagByID is a helper method to get flag by ID
func (r *FlagRepositoryAdapter) GetFlagByID(ctx context.Context, flagID uuid.UUID) (*flags.Flag, error) {
	var pgFlagID pgtype.UUID
	if err := pgFlagID.Scan(flagID); err != nil {
		return nil, fmt.Errorf("invalid flag ID: %v", err)
	}

	dbFlag, err := r.queries.GetFlagByID(ctx, pgFlagID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("flag not found")
		}
		return nil, fmt.Errorf("failed to get flag: %v", err)
	}

	return r.convertDBFlagToCore(dbFlag)
}

// CreateFlagRule creates a new flag rule
func (r *FlagRepositoryAdapter) CreateFlagRule(ctx context.Context, rule *flags.FlagRule) error {
	var pgFlagID pgtype.UUID
	if err := pgFlagID.Scan(rule.FlagID); err != nil {
		return fmt.Errorf("invalid flag ID: %v", err)
	}

	variant, err := json.Marshal(rule.Variant)
	if err != nil {
		return fmt.Errorf("failed to marshal variant: %v", err)
	}

	dbRule, err := r.queries.CreateFlagRule(ctx, CreateFlagRuleParams{
		FlagID:   pgFlagID,
		Priority: int32(rule.Priority),
		Rollout:  int32(rule.Rollout),
		Variant:  variant,
	})
	if err != nil {
		return fmt.Errorf("failed to create flag rule: %v", err)
	}

	// Update the rule with the generated ID
	ruleUUID, err := uuid.FromBytes(dbRule.ID.Bytes[:])
	if err != nil {
		return fmt.Errorf("failed to convert rule ID: %v", err)
	}
	rule.ID = ruleUUID

	return nil
}

// convertDBFlagToCore converts a database flag to core domain model
func (r *FlagRepositoryAdapter) convertDBFlagToCore(dbFlag Flag) (*flags.Flag, error) {
	flagUUID, err := uuid.FromBytes(dbFlag.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert flag ID: %v", err)
	}

	tenantUUID, err := uuid.FromBytes(dbFlag.TenantID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert tenant ID: %v", err)
	}

	flag := &flags.Flag{
		ID:       flagUUID,
		TenantID: tenantUUID,
		Key:      dbFlag.Key,
		Type:     flags.FlagType(dbFlag.Type),
		Salt:     dbFlag.Salt,
	}

	if dbFlag.Description.Valid {
		flag.Description = dbFlag.Description.String
	}

	if dbFlag.Enabled.Valid {
		flag.Enabled = dbFlag.Enabled.Bool
	}

	if dbFlag.CreatedAt.Valid {
		flag.CreatedAt = dbFlag.CreatedAt.Time
	}

	if dbFlag.UpdatedAt.Valid {
		flag.UpdatedAt = dbFlag.UpdatedAt.Time
	}

	return flag, nil
}

// convertDBRuleToCore converts a database flag rule to core domain model
func (r *FlagRepositoryAdapter) convertDBRuleToCore(dbRule FlagRule) (*flags.FlagRule, error) {
	ruleUUID, err := uuid.FromBytes(dbRule.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert rule ID: %v", err)
	}

	flagUUID, err := uuid.FromBytes(dbRule.FlagID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert flag ID: %v", err)
	}

	rule := &flags.FlagRule{
		ID:       ruleUUID,
		FlagID:   flagUUID,
		Priority: int(dbRule.Priority),
		Rollout:  int(dbRule.Rollout),
		Variant:  json.RawMessage(dbRule.Variant),
	}

	return rule, nil
}