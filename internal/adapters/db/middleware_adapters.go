package db

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"
    "github.com/latoulicious/RoutePilot/internal/transport/http/middleware"
)

// APIKeyRepositoryAdapter adapts the database repository for middleware use
type APIKeyRepositoryAdapter struct {
	queries *Queries
}

// NewAPIKeyRepositoryAdapter creates a new API key repository adapter
func NewAPIKeyRepositoryAdapter(queries *Queries) *APIKeyRepositoryAdapter {
	return &APIKeyRepositoryAdapter{
		queries: queries,
	}
}

// GetAPIKeyByID retrieves an API key by ID
func (r *APIKeyRepositoryAdapter) GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (*middleware.APIKey, error) {
	var pgKeyID pgtype.UUID
	if err := pgKeyID.Scan(keyID); err != nil {
		return nil, fmt.Errorf("invalid key ID: %v", err)
	}

	dbKey, err := r.queries.GetAPIKey(ctx, pgKeyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("API key not found")
		}
		return nil, fmt.Errorf("failed to get API key: %v", err)
	}

	// Convert database model to middleware model
	apiKey := &middleware.APIKey{
		ID:        keyID,
		Name:      dbKey.Name,
		SecretEnc: dbKey.SecretEnc,
		Active:    dbKey.Active.Bool,
	}

	// Convert tenant ID
	if dbKey.TenantID.Valid {
		tenantUUID, err := uuid.FromBytes(dbKey.TenantID.Bytes[:])
		if err == nil {
			apiKey.TenantID = tenantUUID
		}
	}

	// Convert created at
	if dbKey.CreatedAt.Valid {
		apiKey.CreatedAt = dbKey.CreatedAt.Time
	}

	return apiKey, nil
}

// UpdateAPIKeyLastUsed updates the last used timestamp for an API key
func (r *APIKeyRepositoryAdapter) UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error {
	var pgKeyID pgtype.UUID
	if err := pgKeyID.Scan(keyID); err != nil {
		return fmt.Errorf("invalid key ID: %v", err)
	}

	return r.queries.UpdateAPIKeyLastUsed(ctx, pgKeyID)
}

// IdempotencyRepositoryAdapter adapts the database repository for middleware use
type IdempotencyRepositoryAdapter struct {
    queries *Queries
}

// NewIdempotencyRepositoryAdapter creates a new idempotency repository adapter
func NewIdempotencyRepositoryAdapter(queries *Queries) *IdempotencyRepositoryAdapter {
	return &IdempotencyRepositoryAdapter{
		queries: queries,
	}
}

// GetIdempotencyKey retrieves an idempotency key
func (r *IdempotencyRepositoryAdapter) GetIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID) (*middleware.IdempotencyKey, error) {
	var pgTenantID, pgKey pgtype.UUID
	if err := pgTenantID.Scan(tenantID); err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %v", err)
	}
	if err := pgKey.Scan(key); err != nil {
		return nil, fmt.Errorf("invalid key: %v", err)
	}

	dbKey, err := r.queries.GetIdempotencyKey(ctx, GetIdempotencyKeyParams{
		TenantID: pgTenantID,
		Key:      pgKey,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("idempotency key not found")
		}
		return nil, fmt.Errorf("failed to get idempotency key: %v", err)
	}

	// Convert database model to middleware model
	idempotencyKey := &middleware.IdempotencyKey{
		TenantID: tenantID,
		Key:      key,
		Method:   dbKey.Method,
		PathHash: dbKey.PathHash,
		Status:   dbKey.Status,
	}

	// Convert created at
	if dbKey.CreatedAt.Valid {
		idempotencyKey.CreatedAt = dbKey.CreatedAt.Time
	}

	return idempotencyKey, nil
}

// CreateIdempotencyKey creates a new idempotency key
func (r *IdempotencyRepositoryAdapter) CreateIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID, method string, pathHash []byte, status int32) (*middleware.IdempotencyKey, error) {
	var pgTenantID, pgKey pgtype.UUID
	if err := pgTenantID.Scan(tenantID); err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %v", err)
	}
	if err := pgKey.Scan(key); err != nil {
		return nil, fmt.Errorf("invalid key: %v", err)
	}

	dbKey, err := r.queries.CreateIdempotencyKey(ctx, CreateIdempotencyKeyParams{
		TenantID: pgTenantID,
		Key:      pgKey,
		Method:   method,
		PathHash: pathHash,
		Status:   status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create idempotency key: %v", err)
	}

	// Convert database model to middleware model
	idempotencyKey := &middleware.IdempotencyKey{
		TenantID: tenantID,
		Key:      key,
		Method:   dbKey.Method,
		PathHash: dbKey.PathHash,
		Status:   dbKey.Status,
	}

	// Convert created at
	if dbKey.CreatedAt.Valid {
		idempotencyKey.CreatedAt = dbKey.CreatedAt.Time
	}

	return idempotencyKey, nil
}

// UpdateIdempotencyStatus updates stored status for an idempotency key
func (r *IdempotencyRepositoryAdapter) UpdateIdempotencyStatus(ctx context.Context, tenantID, key uuid.UUID, status int32) error {
    var pgTenantID, pgKey pgtype.UUID
    if err := pgTenantID.Scan(tenantID); err != nil {
        return fmt.Errorf("invalid tenant ID: %v", err)
    }
    if err := pgKey.Scan(key); err != nil {
        return fmt.Errorf("invalid key: %v", err)
    }
    return r.queries.UpdateIdempotencyStatus(ctx, UpdateIdempotencyStatusParams{
        TenantID: pgTenantID,
        Key:      pgKey,
        Status:   status,
    })
}
