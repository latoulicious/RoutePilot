package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// TestRepository_Structure tests the repository structure and interface compliance
func TestRepository_Structure(t *testing.T) {
	t.Run("NewRepository", func(t *testing.T) {
		// Test that NewRepository creates a valid repository
		// This test doesn't require a real database connection
		repo := &Repository{
			Queries: &Queries{},
			pool:    nil, // nil is fine for structure testing
		}
		
		assert.NotNil(t, repo)
		assert.NotNil(t, repo.Queries)
	})

	t.Run("QuerierInterface", func(t *testing.T) {
		// Test that Queries implements the Querier interface
		var _ Querier = (*Queries)(nil)
		
		// This ensures all required methods are implemented
		assert.True(t, true) // If compilation passes, interface is satisfied
	})
}

// TestRepository_ParameterTypes tests that parameter types are correctly defined
func TestRepository_ParameterTypes(t *testing.T) {
	t.Run("CreateFlagParams", func(t *testing.T) {
		params := CreateFlagParams{
			TenantID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			Key:  "test-flag",
			Type: FlagTypeBoolean,
			Enabled: pgtype.Bool{
				Bool:  true,
				Valid: true,
			},
			Salt: "test-salt",
		}

		assert.Equal(t, "test-flag", params.Key)
		assert.Equal(t, FlagTypeBoolean, params.Type)
		assert.True(t, params.Enabled.Bool)
		assert.Equal(t, "test-salt", params.Salt)
		assert.True(t, params.TenantID.Valid)
	})

	t.Run("GetFlagByTenantKeyParams", func(t *testing.T) {
		params := GetFlagByTenantKeyParams{
			TenantID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			Key: "test-flag-key",
		}

		assert.Equal(t, "test-flag-key", params.Key)
		assert.True(t, params.TenantID.Valid)
	})

	t.Run("UpsertAssignmentParams", func(t *testing.T) {
		params := UpsertAssignmentParams{
			FlagID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			SubjectID:     "user-123",
			Bucket:        1234,
			ChosenVariant: []byte(`{"enabled": true}`),
		}

		assert.Equal(t, "user-123", params.SubjectID)
		assert.Equal(t, int32(1234), params.Bucket)
		assert.Equal(t, `{"enabled": true}`, string(params.ChosenVariant))
		assert.True(t, params.FlagID.Valid)
	})

	t.Run("AddOutboxParams", func(t *testing.T) {
		params := AddOutboxParams{
			TenantID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			Topic: "flag-evaluations",
			Key: pgtype.Text{
				String: "event-key",
				Valid:  true,
			},
			Payload: []byte(`{"event": "test"}`),
		}

		assert.Equal(t, "flag-evaluations", params.Topic)
		assert.Equal(t, "event-key", params.Key.String)
		assert.True(t, params.Key.Valid)
		assert.Equal(t, `{"event": "test"}`, string(params.Payload))
		assert.True(t, params.TenantID.Valid)
	})
}

// TestRepository_ModelTypes tests the generated model types
func TestRepository_ModelTypes(t *testing.T) {
	t.Run("Flag", func(t *testing.T) {
		flag := Flag{
			ID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			TenantID: pgtype.UUID{
				Bytes: [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
				Valid: true,
			},
			Key:  "test-flag",
			Type: FlagTypeJson,
			Enabled: pgtype.Bool{
				Bool:  false,
				Valid: true,
			},
			Salt: "flag-salt",
		}

		assert.True(t, flag.ID.Valid)
		assert.True(t, flag.TenantID.Valid)
		assert.Equal(t, "test-flag", flag.Key)
		assert.Equal(t, FlagTypeJson, flag.Type)
		assert.False(t, flag.Enabled.Bool)
		assert.Equal(t, "flag-salt", flag.Salt)
	})

	t.Run("FlagRule", func(t *testing.T) {
		rule := FlagRule{
			ID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			FlagID:   pgtype.UUID{Valid: true},
			Priority: 1,
			Rollout:  75,
			Variant:  []byte(`{"color": "blue"}`),
		}

		assert.True(t, rule.ID.Valid)
		assert.True(t, rule.FlagID.Valid)
		assert.Equal(t, int32(1), rule.Priority)
		assert.Equal(t, int32(75), rule.Rollout)
		assert.Equal(t, `{"color": "blue"}`, string(rule.Variant))
	})

	t.Run("FlagAssignment", func(t *testing.T) {
		assignment := FlagAssignment{
			ID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			FlagID:        pgtype.UUID{Valid: true},
			SubjectID:     "user-456",
			Bucket:        2500,
			ChosenVariant: []byte(`{"theme": "dark"}`),
		}

		assert.True(t, assignment.ID.Valid)
		assert.True(t, assignment.FlagID.Valid)
		assert.Equal(t, "user-456", assignment.SubjectID)
		assert.Equal(t, int32(2500), assignment.Bucket)
		assert.Equal(t, `{"theme": "dark"}`, string(assignment.ChosenVariant))
	})

	t.Run("Outbox", func(t *testing.T) {
		outbox := Outbox{
			ID: pgtype.UUID{
				Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				Valid: true,
			},
			TenantID: pgtype.UUID{Valid: true},
			Topic:    "conversions",
			Key: pgtype.Text{
				String: "conversion-key",
				Valid:  true,
			},
			Payload: []byte(`{"conversion_id": "conv-123"}`),
		}

		assert.True(t, outbox.ID.Valid)
		assert.True(t, outbox.TenantID.Valid)
		assert.Equal(t, "conversions", outbox.Topic)
		assert.Equal(t, "conversion-key", outbox.Key.String)
		assert.True(t, outbox.Key.Valid)
		assert.Equal(t, `{"conversion_id": "conv-123"}`, string(outbox.Payload))
	})
}

// TestRepository_EnumTypes tests the enum type definitions
func TestRepository_EnumTypes(t *testing.T) {
	t.Run("FlagType", func(t *testing.T) {
		assert.Equal(t, FlagType("boolean"), FlagTypeBoolean)
		assert.Equal(t, FlagType("json"), FlagTypeJson)
	})

	t.Run("ExperimentStatus", func(t *testing.T) {
		assert.Equal(t, ExperimentStatus("draft"), ExperimentStatusDraft)
		assert.Equal(t, ExperimentStatus("running"), ExperimentStatusRunning)
		assert.Equal(t, ExperimentStatus("paused"), ExperimentStatusPaused)
		assert.Equal(t, ExperimentStatus("stopped"), ExperimentStatusStopped)
	})
}

// TestRepository_RequiredQueries verifies all required queries are available
func TestRepository_RequiredQueries(t *testing.T) {
	// Create a mock queries instance to test method signatures
	queries := &Queries{}

	t.Run("FlagQueries", func(t *testing.T) {
		// Test that required flag query methods exist with correct signatures
		var _ func(context.Context, GetFlagByTenantKeyParams) (Flag, error) = queries.GetFlagByTenantKey
		var _ func(context.Context, CreateFlagParams) (Flag, error) = queries.CreateFlag
		var _ func(context.Context, UpdateFlagParams) (Flag, error) = queries.UpdateFlag
		var _ func(context.Context, pgtype.UUID) ([]Flag, error) = queries.ListFlagsByTenant
	})

	t.Run("FlagRuleQueries", func(t *testing.T) {
		// Test that required flag rule query methods exist
		var _ func(context.Context, pgtype.UUID) ([]FlagRule, error) = queries.GetRulesByFlag
		var _ func(context.Context, CreateFlagRuleParams) (FlagRule, error) = queries.CreateFlagRule
		var _ func(context.Context, UpdateFlagRuleParams) (FlagRule, error) = queries.UpdateFlagRule
		var _ func(context.Context, DeleteFlagRuleParams) error = queries.DeleteFlagRule
	})

	t.Run("AssignmentQueries", func(t *testing.T) {
		// Test that required assignment query methods exist
		var _ func(context.Context, GetAssignmentParams) (FlagAssignment, error) = queries.GetAssignment
		var _ func(context.Context, UpsertAssignmentParams) (FlagAssignment, error) = queries.UpsertAssignment
	})

	t.Run("OutboxQueries", func(t *testing.T) {
		// Test that required outbox query methods exist
		var _ func(context.Context, AddOutboxParams) (Outbox, error) = queries.AddOutbox
		var _ func(context.Context, int32) ([]Outbox, error) = queries.ClaimOutboxBatch
		var _ func(context.Context, []pgtype.UUID) error = queries.MarkOutboxPublished
	})
}

// TestRepository_TransactionMethods tests transaction-related methods
func TestRepository_TransactionMethods(t *testing.T) {
	t.Run("WithTx_MethodExists", func(t *testing.T) {
		repo := &Repository{}
		
		// Test that WithTx method exists with correct signature
		var _ func(context.Context, func(*Queries) error) error = repo.WithTx
	})

	t.Run("BeginTx_MethodExists", func(t *testing.T) {
		repo := &Repository{}
		
		// Test that BeginTx method exists with correct signature  
		var _ func(context.Context) (interface{}, *Queries, error) = func(ctx context.Context) (interface{}, *Queries, error) {
			tx, qtx, err := repo.BeginTx(ctx)
			return tx, qtx, err
		}
	})
}