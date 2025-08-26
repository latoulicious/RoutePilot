package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepository_FlagOperations tests basic flag CRUD operations
func TestRepository_FlagOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	repo := setupTestRepository(t)
	
	// Create a test tenant first
	tenant, err := repo.CreateTenant(ctx, "test-tenant")
	require.NoError(t, err)
	require.NotEmpty(t, tenant.ID.Bytes)

	t.Run("CreateFlag", func(t *testing.T) {
		flag, err := repo.CreateFlag(ctx, CreateFlagParams{
			TenantID: tenant.ID,
			Key:      "test-flag",
			Description: pgtype.Text{
				String: "Test flag description",
				Valid:  true,
			},
			Type: FlagTypeBoolean,
			Enabled: pgtype.Bool{
				Bool:  true,
				Valid: true,
			},
			Salt: "test-salt-123",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, flag.ID.Bytes)
		assert.Equal(t, tenant.ID.Bytes, flag.TenantID.Bytes)
		assert.Equal(t, "test-flag", flag.Key)
		assert.Equal(t, "Test flag description", flag.Description.String)
		assert.Equal(t, FlagTypeBoolean, flag.Type)
		assert.True(t, flag.Enabled.Bool)
		assert.Equal(t, "test-salt-123", flag.Salt)
	})

	t.Run("GetFlagByTenantKey", func(t *testing.T) {
		// First create a flag
		createdFlag, err := repo.CreateFlag(ctx, CreateFlagParams{
			TenantID: tenant.ID,
			Key:      "get-test-flag",
			Description: pgtype.Text{
				String: "Get test flag",
				Valid:  true,
			},
			Type: FlagTypeJson,
			Enabled: pgtype.Bool{
				Bool:  false,
				Valid: true,
			},
			Salt: "get-test-salt",
		})
		require.NoError(t, err)

		// Now retrieve it
		retrievedFlag, err := repo.GetFlagByTenantKey(ctx, GetFlagByTenantKeyParams{
			TenantID: tenant.ID,
			Key:      "get-test-flag",
		})

		require.NoError(t, err)
		assert.Equal(t, createdFlag.ID.Bytes, retrievedFlag.ID.Bytes)
		assert.Equal(t, "get-test-flag", retrievedFlag.Key)
		assert.Equal(t, FlagTypeJson, retrievedFlag.Type)
		assert.False(t, retrievedFlag.Enabled.Bool)
	})

	t.Run("UpdateFlag", func(t *testing.T) {
		// First create a flag
		createdFlag, err := repo.CreateFlag(ctx, CreateFlagParams{
			TenantID: tenant.ID,
			Key:      "update-test-flag",
			Description: pgtype.Text{
				String: "Original description",
				Valid:  true,
			},
			Type: FlagTypeBoolean,
			Enabled: pgtype.Bool{
				Bool:  false,
				Valid: true,
			},
			Salt: "original-salt",
		})
		require.NoError(t, err)

		// Update the flag
		updatedFlag, err := repo.UpdateFlag(ctx, UpdateFlagParams{
			TenantID: tenant.ID,
			Key:      "update-test-flag",
			Enabled: pgtype.Bool{
				Bool:  true,
				Valid: true,
			},
			Salt: "new-salt",
			Description: pgtype.Text{
				String: "Updated description",
				Valid:  true,
			},
		})

		require.NoError(t, err)
		assert.Equal(t, createdFlag.ID.Bytes, updatedFlag.ID.Bytes)
		assert.True(t, updatedFlag.Enabled.Bool)
		assert.Equal(t, "new-salt", updatedFlag.Salt)
		assert.Equal(t, "Updated description", updatedFlag.Description.String)
	})
}

// TestRepository_FlagRuleOperations tests flag rule operations
func TestRepository_FlagRuleOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	repo := setupTestRepository(t)
	
	// Create test tenant and flag
	tenant, err := repo.CreateTenant(ctx, "rule-test-tenant")
	require.NoError(t, err)

	flag, err := repo.CreateFlag(ctx, CreateFlagParams{
		TenantID: tenant.ID,
		Key:      "rule-test-flag",
		Type:     FlagTypeBoolean,
		Enabled: pgtype.Bool{
			Bool:  true,
			Valid: true,
		},
		Salt: "rule-test-salt",
	})
	require.NoError(t, err)

	t.Run("CreateFlagRule", func(t *testing.T) {
		variant := map[string]interface{}{
			"enabled": true,
			"value":   "test-value",
		}
		variantJSON, err := json.Marshal(variant)
		require.NoError(t, err)

		rule, err := repo.CreateFlagRule(ctx, CreateFlagRuleParams{
			FlagID:   flag.ID,
			Priority: 1,
			Rollout:  50,
			Variant:  variantJSON,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, rule.ID.Bytes)
		assert.Equal(t, flag.ID.Bytes, rule.FlagID.Bytes)
		assert.Equal(t, int32(1), rule.Priority)
		assert.Equal(t, int32(50), rule.Rollout)
		assert.JSONEq(t, string(variantJSON), string(rule.Variant))
	})

	t.Run("GetRulesByFlag", func(t *testing.T) {
		// Create multiple rules with different priorities
		for i := 1; i <= 3; i++ {
			variant := map[string]interface{}{
				"priority": i,
				"enabled":  i%2 == 0,
			}
			variantJSON, err := json.Marshal(variant)
			require.NoError(t, err)

			_, err = repo.CreateFlagRule(ctx, CreateFlagRuleParams{
				FlagID:   flag.ID,
				Priority: int32(i + 10), // Use different priorities to avoid conflicts
				Rollout:  int32(i * 20),
				Variant:  variantJSON,
			})
			require.NoError(t, err)
		}

		// Retrieve all rules for the flag
		rules, err := repo.GetRulesByFlag(ctx, flag.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rules), 3)

		// Verify rules are ordered by priority
		for i := 1; i < len(rules); i++ {
			assert.LessOrEqual(t, rules[i-1].Priority, rules[i].Priority)
		}
	})
}

// TestRepository_AssignmentOperations tests assignment operations
func TestRepository_AssignmentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	repo := setupTestRepository(t)
	
	// Create test tenant and flag
	tenant, err := repo.CreateTenant(ctx, "assignment-test-tenant")
	require.NoError(t, err)

	flag, err := repo.CreateFlag(ctx, CreateFlagParams{
		TenantID: tenant.ID,
		Key:      "assignment-test-flag",
		Type:     FlagTypeJson,
		Enabled: pgtype.Bool{
			Bool:  true,
			Valid: true,
		},
		Salt: "assignment-test-salt",
	})
	require.NoError(t, err)

	t.Run("UpsertAssignment_Create", func(t *testing.T) {
		variant := map[string]interface{}{
			"color": "blue",
			"size":  "large",
		}
		variantJSON, err := json.Marshal(variant)
		require.NoError(t, err)

		assignment, err := repo.UpsertAssignment(ctx, UpsertAssignmentParams{
			FlagID:        flag.ID,
			SubjectID:     "user-123",
			Bucket:        1234,
			ChosenVariant: variantJSON,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, assignment.ID.Bytes)
		assert.Equal(t, flag.ID.Bytes, assignment.FlagID.Bytes)
		assert.Equal(t, "user-123", assignment.SubjectID)
		assert.Equal(t, int32(1234), assignment.Bucket)
		assert.JSONEq(t, string(variantJSON), string(assignment.ChosenVariant))
	})

	t.Run("GetAssignment", func(t *testing.T) {
		// First create an assignment
		variant := map[string]interface{}{
			"theme": "dark",
		}
		variantJSON, err := json.Marshal(variant)
		require.NoError(t, err)

		createdAssignment, err := repo.UpsertAssignment(ctx, UpsertAssignmentParams{
			FlagID:        flag.ID,
			SubjectID:     "user-456",
			Bucket:        5678,
			ChosenVariant: variantJSON,
		})
		require.NoError(t, err)

		// Now retrieve it
		retrievedAssignment, err := repo.GetAssignment(ctx, GetAssignmentParams{
			FlagID:    flag.ID,
			SubjectID: "user-456",
		})

		require.NoError(t, err)
		assert.Equal(t, createdAssignment.ID.Bytes, retrievedAssignment.ID.Bytes)
		assert.Equal(t, "user-456", retrievedAssignment.SubjectID)
		assert.Equal(t, int32(5678), retrievedAssignment.Bucket)
		assert.JSONEq(t, string(variantJSON), string(retrievedAssignment.ChosenVariant))
	})

	t.Run("UpsertAssignment_Update", func(t *testing.T) {
		// Create initial assignment
		initialVariant := map[string]interface{}{
			"version": "v1",
		}
		initialJSON, err := json.Marshal(initialVariant)
		require.NoError(t, err)

		initial, err := repo.UpsertAssignment(ctx, UpsertAssignmentParams{
			FlagID:        flag.ID,
			SubjectID:     "user-update-test",
			Bucket:        1111,
			ChosenVariant: initialJSON,
		})
		require.NoError(t, err)

		// Update the same assignment
		updatedVariant := map[string]interface{}{
			"version": "v2",
		}
		updatedJSON, err := json.Marshal(updatedVariant)
		require.NoError(t, err)

		updated, err := repo.UpsertAssignment(ctx, UpsertAssignmentParams{
			FlagID:        flag.ID,
			SubjectID:     "user-update-test",
			Bucket:        2222,
			ChosenVariant: updatedJSON,
		})

		require.NoError(t, err)
		assert.Equal(t, initial.ID.Bytes, updated.ID.Bytes) // Same ID
		assert.Equal(t, int32(2222), updated.Bucket)        // Updated bucket
		assert.JSONEq(t, string(updatedJSON), string(updated.ChosenVariant))
		assert.True(t, updated.AssignedAt.Time.After(initial.AssignedAt.Time)) // Updated timestamp
	})
}

// TestRepository_OutboxOperations tests outbox operations
func TestRepository_OutboxOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	repo := setupTestRepository(t)
	
	// Create test tenant
	tenant, err := repo.CreateTenant(ctx, "outbox-test-tenant")
	require.NoError(t, err)

	t.Run("AddOutbox", func(t *testing.T) {
		payload := map[string]interface{}{
			"event_type": "flag_evaluation",
			"flag_key":   "test-flag",
			"user_id":    "user-123",
			"result":     true,
		}
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		event, err := repo.AddOutbox(ctx, AddOutboxParams{
			TenantID: tenant.ID,
			Topic:    "flag-evaluations",
			Key: pgtype.Text{
				String: "user-123",
				Valid:  true,
			},
			Payload: payloadJSON,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, event.ID.Bytes)
		assert.Equal(t, tenant.ID.Bytes, event.TenantID.Bytes)
		assert.Equal(t, "flag-evaluations", event.Topic)
		assert.Equal(t, "user-123", event.Key.String)
		assert.JSONEq(t, string(payloadJSON), string(event.Payload))
		assert.False(t, event.PublishedAt.Valid) // Should not be published yet
	})

	t.Run("ClaimOutboxBatch", func(t *testing.T) {
		// Add multiple events
		for i := 0; i < 5; i++ {
			payload := map[string]interface{}{
				"event_id": i,
				"data":     "test-data",
			}
			payloadJSON, err := json.Marshal(payload)
			require.NoError(t, err)

			_, err = repo.AddOutbox(ctx, AddOutboxParams{
				TenantID: tenant.ID,
				Topic:    "test-events",
				Key: pgtype.Text{
					String: "batch-test",
					Valid:  true,
				},
				Payload: payloadJSON,
			})
			require.NoError(t, err)
		}

		// Claim a batch of 3 events
		events, err := repo.ClaimOutboxBatch(ctx, 3)
		require.NoError(t, err)
		assert.Len(t, events, 3)

		// Verify events are ordered by created_at
		for i := 1; i < len(events); i++ {
			assert.True(t, events[i-1].CreatedAt.Time.Before(events[i].CreatedAt.Time) ||
				events[i-1].CreatedAt.Time.Equal(events[i].CreatedAt.Time))
		}

		// All claimed events should be unpublished
		for _, event := range events {
			assert.False(t, event.PublishedAt.Valid)
		}
	})

	t.Run("MarkOutboxPublished", func(t *testing.T) {
		// Add an event
		payload := map[string]interface{}{
			"test": "publish-test",
		}
		payloadJSON, err := json.Marshal(payload)
		require.NoError(t, err)

		event, err := repo.AddOutbox(ctx, AddOutboxParams{
			TenantID: tenant.ID,
			Topic:    "publish-test",
			Key: pgtype.Text{
				String: "publish-key",
				Valid:  true,
			},
			Payload: payloadJSON,
		})
		require.NoError(t, err)

		// Mark it as published
		err = repo.MarkOutboxPublished(ctx, []pgtype.UUID{event.ID})
		require.NoError(t, err)

		// Verify it's no longer returned in batch claims
		events, err := repo.ClaimOutboxBatch(ctx, 10)
		require.NoError(t, err)
		
		// The published event should not be in the batch
		for _, e := range events {
			assert.NotEqual(t, event.ID.Bytes, e.ID.Bytes)
		}
	})
}

// TestRepository_TransactionSupport tests transaction functionality
func TestRepository_TransactionSupport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	repo := setupTestRepository(t)
	
	tenant, err := repo.CreateTenant(ctx, "tx-test-tenant")
	require.NoError(t, err)

	t.Run("WithTx_Success", func(t *testing.T) {
		var createdFlag Flag
		
		err := repo.WithTx(ctx, func(qtx *Queries) error {
			flag, err := qtx.CreateFlag(ctx, CreateFlagParams{
				TenantID: tenant.ID,
				Key:      "tx-success-flag",
				Type:     FlagTypeBoolean,
				Enabled: pgtype.Bool{
					Bool:  true,
					Valid: true,
				},
				Salt: "tx-success-salt",
			})
			if err != nil {
				return err
			}
			createdFlag = flag
			return nil
		})

		require.NoError(t, err)
		
		// Verify the flag was created
		retrievedFlag, err := repo.GetFlagByTenantKey(ctx, GetFlagByTenantKeyParams{
			TenantID: tenant.ID,
			Key:      "tx-success-flag",
		})
		require.NoError(t, err)
		assert.Equal(t, createdFlag.ID.Bytes, retrievedFlag.ID.Bytes)
	})

	t.Run("WithTx_Rollback", func(t *testing.T) {
		err := repo.WithTx(ctx, func(qtx *Queries) error {
			// Create a flag
			_, err := qtx.CreateFlag(ctx, CreateFlagParams{
				TenantID: tenant.ID,
				Key:      "tx-rollback-flag",
				Type:     FlagTypeBoolean,
				Enabled: pgtype.Bool{
					Bool:  true,
					Valid: true,
				},
				Salt: "tx-rollback-salt",
			})
			if err != nil {
				return err
			}
			
			// Force an error to trigger rollback
			return assert.AnError
		})

		require.Error(t, err)
		assert.Equal(t, assert.AnError, err)
		
		// Verify the flag was not created due to rollback
		_, err = repo.GetFlagByTenantKey(ctx, GetFlagByTenantKeyParams{
			TenantID: tenant.ID,
			Key:      "tx-rollback-flag",
		})
		require.Error(t, err) // Should not exist
	})
}

// setupTestRepository creates a test repository with a real database connection
// This requires a test database to be available
func setupTestRepository(t *testing.T) *Repository {
	// This would typically use a test database URL from environment
	// For now, we'll skip if no database is available
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("No test database URL provided")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	
	// Clean up function
	t.Cleanup(func() {
		pool.Close()
	})

	return NewRepository(pool)
}

// getTestDatabaseURL returns the test database URL from environment
// In a real implementation, this would read from TEST_DATABASE_URL env var
func getTestDatabaseURL() string {
	// For now, return empty to skip tests that require a database
	// In practice, you'd use: os.Getenv("TEST_DATABASE_URL")
	return ""
}