package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
)

// CacheAwareFlagRepositoryAdapter wraps the FlagRepositoryAdapter with cache invalidation
type CacheAwareFlagRepositoryAdapter struct {
	*FlagRepositoryAdapter
	cache *flags.FlagRulesCache
}

// NewCacheAwareFlagRepositoryAdapter creates a new cache-aware flag repository adapter
func NewCacheAwareFlagRepositoryAdapter(queries *Queries, cache *flags.FlagRulesCache) *CacheAwareFlagRepositoryAdapter {
	return &CacheAwareFlagRepositoryAdapter{
		FlagRepositoryAdapter: NewFlagRepositoryAdapter(queries),
		cache:                 cache,
	}
}

// Ensure CacheAwareFlagRepositoryAdapter implements the interface
var _ ports.FlagRepository = (*CacheAwareFlagRepositoryAdapter)(nil)

// UpdateFlag updates a flag and invalidates its cache entry
func (r *CacheAwareFlagRepositoryAdapter) UpdateFlag(ctx context.Context, flagID uuid.UUID, updates flags.FlagUpdates) error {
	// Update the flag
	err := r.FlagRepositoryAdapter.UpdateFlag(ctx, flagID, updates)
	if err != nil {
		return err
	}

	// Invalidate cache entry for this flag
	r.cache.Invalidate(flagID)

	return nil
}

// CreateFlagRule creates a flag rule and invalidates the flag's cache entry
func (r *CacheAwareFlagRepositoryAdapter) CreateFlagRule(ctx context.Context, rule *flags.FlagRule) error {
	// Create the rule
	err := r.FlagRepositoryAdapter.CreateFlagRule(ctx, rule)
	if err != nil {
		return err
	}

	// Invalidate cache entry for this flag since rules changed
	r.cache.Invalidate(rule.FlagID)

	return nil
}

// GetCache returns the underlying cache for direct access if needed
func (r *CacheAwareFlagRepositoryAdapter) GetCache() *flags.FlagRulesCache {
	return r.cache
}
