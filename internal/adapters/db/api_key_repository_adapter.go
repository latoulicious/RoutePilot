package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/latoulicious/RoutePilot/internal/domain/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
)

// APIKeysRepositoryAdapter implements ports.APIKeyRepository for CLI/admin use
type APIKeysRepositoryAdapter struct {
	queries *Queries
}

var _ ports.APIKeyRepository = (*APIKeysRepositoryAdapter)(nil)

func NewAPIKeysRepositoryAdapter(q *Queries) *APIKeysRepositoryAdapter {
	return &APIKeysRepositoryAdapter{queries: q}
}

func (r *APIKeysRepositoryAdapter) GetAPIKeyByKeyID(ctx context.Context, keyID string) (*flags.APIKey, error) {
	dbKey, err := r.queries.GetAPIKeyByKeyID(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key by key ID: %w", err)
	}

	id, err := uuid.FromBytes(dbKey.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("invalid API key ID: %w", err)
	}

	tenantID, err := uuid.FromBytes(dbKey.TenantID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	return &flags.APIKey{
		ID:           id,
		TenantID:     tenantID,
		Name:         dbKey.Name,
		KeyID:        dbKey.KeyID,
		SecretHash:   dbKey.SecretHash,
		EncryptedKey: string(dbKey.SecretEnc),
		CreatedAt:    dbKey.CreatedAt.Time,
		UpdatedAt:    dbKey.LastUsedAt.Time,
	}, nil
}

func (r *APIKeysRepositoryAdapter) CreateAPIKey(ctx context.Context, apiKey *flags.APIKey) error {
	var pgTenantID pgtype.UUID
	if err := pgTenantID.Scan(apiKey.TenantID); err != nil {
		return fmt.Errorf("invalid tenant ID: %v", err)
	}

	created, err := r.queries.CreateAPIKey(ctx, CreateAPIKeyParams{
		TenantID:   pgTenantID,
		Name:       apiKey.Name,
		KeyID:      apiKey.KeyID,
		SecretHash: apiKey.SecretHash,
		SecretEnc:  []byte(apiKey.EncryptedKey),
	})
	if err != nil {
		return fmt.Errorf("failed to create api key: %w", err)
	}

	id, err := uuid.FromBytes(created.ID.Bytes[:])
	if err == nil {
		apiKey.ID = id
	}
	return nil
}

func (r *APIKeysRepositoryAdapter) DeleteAPIKey(ctx context.Context, keyID string) error {
	// Not implemented in SQL; out of scope for v1
	return fmt.Errorf("DeleteAPIKey not implemented")
}
