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
    // Not needed for current CLI flow
    return nil, fmt.Errorf("GetAPIKeyByKeyID not implemented")
}

func (r *APIKeysRepositoryAdapter) CreateAPIKey(ctx context.Context, apiKey *flags.APIKey) error {
    var pgTenantID pgtype.UUID
    if err := pgTenantID.Scan(apiKey.TenantID); err != nil {
        return fmt.Errorf("invalid tenant ID: %v", err)
    }

    created, err := r.queries.CreateAPIKey(ctx, CreateAPIKeyParams{
        TenantID:  pgTenantID,
        Name:      apiKey.Name,
        SecretEnc: []byte(apiKey.EncryptedKey),
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

