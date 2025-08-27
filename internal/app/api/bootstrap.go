package api

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/latoulicious/RoutePilot/internal/adapters/db"
    "github.com/latoulicious/RoutePilot/internal/config"
    dflags "github.com/latoulicious/RoutePilot/internal/domain/flags"
    "github.com/latoulicious/RoutePilot/internal/ports"
    httpapi "github.com/latoulicious/RoutePilot/internal/transport/http"
    "github.com/latoulicious/RoutePilot/internal/transport/http/middleware"
)

// BuildServer wires dependencies and returns the HTTP server and a cleanup func
func BuildServer(ctx context.Context, cfg config.AppConfig) (*httpapi.Server, func(), error) {
    // Connect DB
    pool, err := db.NewConnection(ctx, cfg.DB.URL)
    if err != nil {
        return nil, nil, fmt.Errorf("db connection failed: %w", err)
    }

    cleanup := func() {
        pool.Close()
    }

    // Queries and base repo
    queries := db.New(pool)
    baseRepo := db.NewRepository(pool)

    // Repositories
    flagRepo := db.NewFlagRepositoryAdapter(queries)
    expRepo := db.NewExperimentRepositoryAdapter(queries)
    outboxRepo := db.NewOutboxRepositoryAdapter(baseRepo)
    apiKeyRepo := db.NewAPIKeyRepositoryAdapter(queries)
    idemRepo := db.NewIdempotencyRepositoryAdapter(queries)

    // Domain cache manager
    cm := dflags.NewCacheManager(flagRepo, dflags.CacheConfig{
        MaxSize:         cfg.Cache.MaxSize,
        TTL:             cfg.Cache.TTL,
        CleanupInterval: cfg.Cache.CleanupInterval,
    })
    cm.StartCleanupRoutine(ctx)

    // Domain evaluator and experiment engine; use an in-memory assignment repo for now
    assignmentRepo := db.NewAssignmentRepositoryAdapter(queries)
    engine := dflags.NewExperimentEngine(expRepo, assignmentRepo)
    evaluator := dflags.NewFlagEvaluator(cm.GetRepository(), assignmentRepo, expRepo, engine)

    // Decryptor (fallback to no-op if missing key)
    var decryptor middleware.SecretDecryptor
    if d, err := middleware.NewAESGCMDecryptor(); err == nil {
        decryptor = d
    } else {
        decryptor = &noopDecryptor{}
    }

    // Cache invalidator adapter by key
    cacheInvalidator := &byKeyCacheInvalidator{flagRepo: flagRepo, cache: cm}

    // Router
    router := httpapi.NewRouter(httpapi.RouterConfig{
        Evaluator:       evaluator,
        OutboxRepo:      outboxRepo,
        FlagRepo:        flagRepo,
        ExperimentRepo:  expRepo,
        Cache:           cacheInvalidator,
        APIKeyRepo:      apiKeyRepo,
        IdempotencyRepo: idemRepo,
        Decryptor:       decryptor,
        RateLimitConfig: &middleware.RateLimitConfig{
            EvalRequests:       cfg.RateLimit.EvalRPS,
            EvalBurst:          cfg.RateLimit.EvalBurst,
            AdminRequests:      cfg.RateLimit.AdminRPS,
            AdminBurst:         cfg.RateLimit.AdminBurst,
            ConversionRequests: cfg.RateLimit.ConvRPS,
            ConversionBurst:    cfg.RateLimit.ConvBurst,
        },
        EvalTimeout:     cfg.EvalTimeout,
    })

    // Server
    server := httpapi.NewServer(router, httpapi.ServerConfig{Port: cfg.HTTP.Port})
    return server, cleanup, nil
}

// byKeyCacheInvalidator adapts key-based invalidation to ID-based cache
type byKeyCacheInvalidator struct {
    flagRepo ports.FlagRepository
    cache    *dflags.CacheManager
}

func (c *byKeyCacheInvalidator) InvalidateFlag(tenantID uuid.UUID, flagKey string) {
    if flag, err := c.flagRepo.GetFlagByKey(context.Background(), tenantID, flagKey); err == nil {
        c.cache.InvalidateFlag(flag.ID)
    }
}

// noopDecryptor returns input as-is (dev fallback)
type noopDecryptor struct{}

func (n *noopDecryptor) Decrypt(b []byte) ([]byte, error) { return b, nil }

// removed in-memory assignment repo; using DB adapter now
