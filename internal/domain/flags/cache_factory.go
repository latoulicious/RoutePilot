package flags

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// CacheConfig holds configuration for the caching layer
type CacheConfig struct {
    MaxSize         int           // Maximum number of entries in cache
    TTL             time.Duration // Time-to-live for cache entries
    CleanupInterval time.Duration // How often to run cleanup of expired entries
}

// DefaultCacheConfig returns a sensible default cache configuration
func DefaultCacheConfig() CacheConfig {
    return CacheConfig{
        MaxSize:         1000,             // 1000 flag rule sets
        TTL:             30 * time.Second, // 30-second TTL as per requirements
        CleanupInterval: 5 * time.Minute,  // Cleanup every 5 minutes
    }
}

// CacheManager manages the lifecycle of the caching layer
type CacheManager struct {
    cache         *FlagRulesCache
    cachedRepo    *CachedFlagRepository
    cleanupCancel context.CancelFunc
    config        CacheConfig
}

// NewCacheManager creates a new cache manager with the given configuration
func NewCacheManager(repo FlagRepositoryInterface, config CacheConfig) *CacheManager {
    cache := NewFlagRulesCache(config.MaxSize, config.TTL)
    cachedRepo := NewCachedFlagRepository(repo, cache)

    return &CacheManager{
        cache:      cache,
        cachedRepo: cachedRepo,
        config:     config,
    }
}

// GetRepository returns the cached repository
func (cm *CacheManager) GetRepository() *CachedFlagRepository {
    return cm.cachedRepo
}

// GetCache returns the underlying cache for direct access
func (cm *CacheManager) GetCache() *FlagRulesCache {
    return cm.cache
}

// StartCleanupRoutine starts the background cleanup routine
func (cm *CacheManager) StartCleanupRoutine(ctx context.Context) {
    cleanupCtx, cancel := context.WithCancel(ctx)
    cm.cleanupCancel = cancel
    cm.cachedRepo.StartCleanupRoutine(cleanupCtx, cm.config.CleanupInterval)
}

// Stop stops the cleanup routine
func (cm *CacheManager) Stop() {
    if cm.cleanupCancel != nil {
        cm.cleanupCancel()
    }
}

// WarmCache pre-loads the cache with flag rules for the given flag IDs
func (cm *CacheManager) WarmCache(ctx context.Context, flagIDs []uuid.UUID) error {
    return cm.cachedRepo.WarmCache(ctx, flagIDs)
}

// InvalidateFlag invalidates the cache entry for a specific flag
func (cm *CacheManager) InvalidateFlag(flagID uuid.UUID) {
    cm.cachedRepo.InvalidateCache(flagID)
}

// ClearCache clears all cache entries
func (cm *CacheManager) ClearCache() {
    cm.cache.Clear()
}

// GetCacheStats returns cache statistics
func (cm *CacheManager) GetCacheStats() CacheStats {
    return CacheStats{
        Size:    cm.cache.Size(),
        MaxSize: cm.config.MaxSize,
        TTL:     cm.config.TTL,
    }
}

// CacheStats holds cache statistics
type CacheStats struct {
    Size    int
    MaxSize int
    TTL     time.Duration
}

