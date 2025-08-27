package config

import (
    "fmt"
    "net"
    "os"
    "strconv"
    "strings"
    "time"
)

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
    Port int
}

// DBConfig holds database configuration
type DBConfig struct {
    URL string
}

// CacheConfig holds domain cache configuration
type CacheConfig struct {
    MaxSize         int
    TTL             time.Duration
    CleanupInterval time.Duration
}

// AppConfig aggregates all app configs
type AppConfig struct {
    HTTP  HTTPConfig
    DB    DBConfig
    Cache CacheConfig
    EvalTimeout time.Duration
    RateLimit   RateLimitConfig
}

// RateLimitConfig holds per-route rate limit settings
type RateLimitConfig struct {
    EvalRPS    int
    EvalBurst  int
    AdminRPS   int
    AdminBurst int
    ConvRPS    int
    ConvBurst  int
}

// Load loads configuration from environment with sane defaults
func Load() (AppConfig, error) {
    cfg := AppConfig{}

    // HTTP: support HTTP_PORT or plan's HTTP_ADDR (host:port)
    port := 8080
    if v := os.Getenv("HTTP_PORT"); v != "" {
        if p, err := strconv.Atoi(v); err == nil {
            port = p
        } else {
            return cfg, fmt.Errorf("invalid HTTP_PORT: %w", err)
        }
    } else if addr := os.Getenv("HTTP_ADDR"); addr != "" {
        // Try to parse :port or host:port
        // If begins with ':', trim leading ':' and parse
        if strings.HasPrefix(addr, ":") {
            if p, err := strconv.Atoi(strings.TrimPrefix(addr, ":")); err == nil {
                port = p
            } else {
                return cfg, fmt.Errorf("invalid HTTP_ADDR port: %w", err)
            }
        } else if _, p, err := net.SplitHostPort(addr); err == nil {
            if pi, err := strconv.Atoi(p); err == nil {
                port = pi
            } else {
                return cfg, fmt.Errorf("invalid HTTP_ADDR port: %w", err)
            }
        } else {
            return cfg, fmt.Errorf("invalid HTTP_ADDR; expected host:port or :port")
        }
    }
    cfg.HTTP = HTTPConfig{Port: port}

    // DB: prefer PG_DSN (plan), fallback to DATABASE_URL
    dbURL := os.Getenv("PG_DSN")
    if dbURL == "" {
        dbURL = os.Getenv("DATABASE_URL")
    }
    if dbURL == "" {
        return cfg, fmt.Errorf("PG_DSN or DATABASE_URL is required")
    }
    cfg.DB = DBConfig{URL: dbURL}

    // Cache
    cfg.Cache = CacheConfig{
        MaxSize:         getInt("CACHE_MAX_SIZE", 1000),
        TTL:             getDuration("CACHE_TTL", 30*time.Second),
        CleanupInterval: getDuration("CACHE_CLEANUP_INTERVAL", 5*time.Minute),
    }

    // Evaluation timeout
    if v := os.Getenv("EVAL_TIMEOUT_MS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            cfg.EvalTimeout = time.Duration(n) * time.Millisecond
        } else {
            return cfg, fmt.Errorf("invalid EVAL_TIMEOUT_MS: %v", err)
        }
    } else {
        cfg.EvalTimeout = 200 * time.Millisecond
    }

    // Rate limiting (defaults per plan)
    cfg.RateLimit = RateLimitConfig{
        EvalRPS:    getInt("RL_EVAL_RPS", 50),
        EvalBurst:  getInt("RL_EVAL_BURST", 100),
        AdminRPS:   getInt("RL_ADMIN_RPS", 10),
        AdminBurst: getInt("RL_ADMIN_BURST", 20),
        ConvRPS:    getInt("RL_CONV_RPS", 100),
        ConvBurst:  getInt("RL_CONV_BURST", 200),
    }

    return cfg, nil
}

func getInt(env string, def int) int {
    if v := os.Getenv(env); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return def
}

func getDuration(env string, def time.Duration) time.Duration {
    if v := os.Getenv(env); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            return d
        }
    }
    return def
}
