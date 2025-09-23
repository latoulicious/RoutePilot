package flags

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
)

// CacheEntry represents a cached item with TTL
type CacheEntry struct {
    Value     []*FlagRule
    ExpiresAt time.Time
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
    return time.Now().After(e.ExpiresAt)
}

// FlagRulesCache provides TTL-based LRU caching for flag rules
type FlagRulesCache struct {
    mu          sync.RWMutex
    cache       map[uuid.UUID]*CacheEntry
    accessOrder []uuid.UUID // LRU tracking
    maxSize     int
    ttl         time.Duration
}

// NewFlagRulesCache creates a new flag rules cache
func NewFlagRulesCache(maxSize int, ttl time.Duration) *FlagRulesCache {
    return &FlagRulesCache{
        cache:       make(map[uuid.UUID]*CacheEntry),
        accessOrder: make([]uuid.UUID, 0, maxSize),
        maxSize:     maxSize,
        ttl:         ttl,
    }
}

// Get retrieves flag rules from cache if available and not expired
func (c *FlagRulesCache) Get(flagID uuid.UUID) ([]*FlagRule, bool) {
    c.mu.RLock()
    entry, exists := c.cache[flagID]
    c.mu.RUnlock()

    if !exists || entry.IsExpired() {
        if exists {
            // Clean up expired entry
            c.mu.Lock()
            delete(c.cache, flagID)
            c.removeFromAccessOrder(flagID)
            c.mu.Unlock()
        }
        return nil, false
    }

    // Update access order for LRU
    c.mu.Lock()
    c.updateAccessOrder(flagID)
    c.mu.Unlock()

    return entry.Value, true
}

// Set stores flag rules in cache with TTL
func (c *FlagRulesCache) Set(flagID uuid.UUID, rules []*FlagRule) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Create new cache entry
    entry := &CacheEntry{
        Value:     rules,
        ExpiresAt: time.Now().Add(c.ttl),
    }

    // Check if we need to evict entries to stay within size limit
    if len(c.cache) >= c.maxSize && c.cache[flagID] == nil {
        c.evictLRU()
    }

    // Store the entry
    c.cache[flagID] = entry
    c.updateAccessOrder(flagID)
}

// Invalidate removes a specific flag's rules from cache
func (c *FlagRulesCache) Invalidate(flagID uuid.UUID) {
    c.mu.Lock()
    defer c.mu.Unlock()

    delete(c.cache, flagID)
    c.removeFromAccessOrder(flagID)
}

// Clear removes all entries from cache
func (c *FlagRulesCache) Clear() {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.cache = make(map[uuid.UUID]*CacheEntry)
    c.accessOrder = c.accessOrder[:0]
}

// Size returns the current number of cached entries
func (c *FlagRulesCache) Size() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return len(c.cache)
}

// CleanupExpired removes all expired entries from cache
func (c *FlagRulesCache) CleanupExpired() int {
    c.mu.Lock()
    defer c.mu.Unlock()

    var expiredKeys []uuid.UUID
    for flagID, entry := range c.cache {
        if entry.IsExpired() {
            expiredKeys = append(expiredKeys, flagID)
        }
    }

    for _, flagID := range expiredKeys {
        delete(c.cache, flagID)
        c.removeFromAccessOrder(flagID)
    }

    return len(expiredKeys)
}

// updateAccessOrder moves the flagID to the end of access order (most recently used)
func (c *FlagRulesCache) updateAccessOrder(flagID uuid.UUID) {
    // Remove from current position if exists
    c.removeFromAccessOrder(flagID)
    // Add to end (most recently used)
    c.accessOrder = append(c.accessOrder, flagID)
}

// removeFromAccessOrder removes flagID from access order slice
func (c *FlagRulesCache) removeFromAccessOrder(flagID uuid.UUID) {
    for i, id := range c.accessOrder {
        if id == flagID {
            // Remove by swapping with last element and truncating
            c.accessOrder[i] = c.accessOrder[len(c.accessOrder)-1]
            c.accessOrder = c.accessOrder[:len(c.accessOrder)-1]
            break
        }
    }
}

// evictLRU removes the least recently used entry
func (c *FlagRulesCache) evictLRU() {
    if len(c.accessOrder) == 0 {
        return
    }

    // First element is least recently used
    lruFlagID := c.accessOrder[0]
    delete(c.cache, lruFlagID)
    c.accessOrder = c.accessOrder[1:]
}

// CachedFlagRepository wraps a FlagRepository with caching
type CachedFlagRepository struct {
    repo  FlagRepositoryInterface
    cache *FlagRulesCache
}

// NewCachedFlagRepository creates a new cached flag repository
func NewCachedFlagRepository(repo FlagRepositoryInterface, cache *FlagRulesCache) *CachedFlagRepository {
    return &CachedFlagRepository{
        repo:  repo,
        cache: cache,
    }
}

// GetFlagByKey delegates to underlying repository (no caching for flags themselves)
func (c *CachedFlagRepository) GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*Flag, error) {
    return c.repo.GetFlagByKey(ctx, tenantID, key)
}

// GetFlagRules retrieves flag rules with caching
func (c *CachedFlagRepository) GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*FlagRule, error) {
    // Try cache first
    if rules, found := c.cache.Get(flagID); found {
        return rules, nil
    }

    // Cache miss - fetch from repository
    rules, err := c.repo.GetFlagRules(ctx, flagID)
    if err != nil {
        return nil, err
    }

    // Store in cache
    c.cache.Set(flagID, rules)

    return rules, nil
}

// InvalidateCache provides a way to invalidate cache entries
func (c *CachedFlagRepository) InvalidateCache(flagID uuid.UUID) {
    c.cache.Invalidate(flagID)
}

// WarmCache pre-loads cache with flag rules for given flag IDs
func (c *CachedFlagRepository) WarmCache(ctx context.Context, flagIDs []uuid.UUID) error {
    for _, flagID := range flagIDs {
        rules, err := c.repo.GetFlagRules(ctx, flagID)
        if err != nil {
            // Log error but continue warming other entries
            continue
        }
        c.cache.Set(flagID, rules)
    }
    return nil
}

// StartCleanupRoutine starts a background goroutine to periodically clean up expired entries
func (c *CachedFlagRepository) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                c.cache.CleanupExpired()
            }
        }
    }()
}

