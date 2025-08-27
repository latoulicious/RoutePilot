package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
)

// TenantRepositoryAdapter adapts the database repository for tenant operations
type TenantRepositoryAdapter struct {
	queries *Queries
}

// Ensure TenantRepositoryAdapter implements the interface
var _ ports.TenantRepository = (*TenantRepositoryAdapter)(nil)

// NewTenantRepositoryAdapter creates a new tenant repository adapter
func NewTenantRepositoryAdapter(queries *Queries) *TenantRepositoryAdapter {
	return &TenantRepositoryAdapter{
		queries: queries,
	}
}

// GetTenantByID retrieves a tenant by ID
func (r *TenantRepositoryAdapter) GetTenantByID(ctx context.Context, tenantID uuid.UUID) (*flags.Tenant, error) {
	pgTenantID := pgtype.UUID{
		Bytes: tenantID,
		Valid: true,
	}

	dbTenant, err := r.queries.GetTenant(ctx, pgTenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	return r.convertDBTenantToCore(dbTenant)
}

// CreateTenant creates a new tenant
func (r *TenantRepositoryAdapter) CreateTenant(ctx context.Context, tenant *flags.Tenant) error {
	dbTenant, err := r.queries.CreateTenant(ctx, tenant.Name)
	if err != nil {
		return fmt.Errorf("failed to create tenant: %w", err)
	}

	// Update the tenant with the generated ID and timestamps
	coreTenant, err := r.convertDBTenantToCore(dbTenant)
	if err != nil {
		return fmt.Errorf("failed to convert created tenant: %w", err)
	}

	// Update the input tenant with the database values
	tenant.ID = coreTenant.ID
	tenant.CreatedAt = coreTenant.CreatedAt

	return nil
}

// convertDBTenantToCore converts a database tenant to core domain model
func (r *TenantRepositoryAdapter) convertDBTenantToCore(dbTenant Tenant) (*flags.Tenant, error) {
	tenantUUID, err := uuid.FromBytes(dbTenant.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to convert tenant ID: %w", err)
	}

	return &flags.Tenant{
		ID:        tenantUUID,
		Name:      dbTenant.Name,
		CreatedAt: dbTenant.CreatedAt.Time,
	}, nil
}
