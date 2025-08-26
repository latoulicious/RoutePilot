package job

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/latoulicious/RoutePilot/internal/adapters/kafka"
)

// WorkerConfig holds all configuration for the outbox worker
type WorkerConfig struct {
	Database *DatabaseConfig
	Kafka    *kafka.Config
	Worker   *OutboxWorkerConfig
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	URL            string
	MaxConnections int
	MaxIdleTime    time.Duration
	MaxLifetime    time.Duration
	ConnectTimeout time.Duration
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() (*WorkerConfig, error) {
	dbConfig, err := loadDatabaseConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load database config: %w", err)
	}

	kafkaConfig, err := loadKafkaConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load Kafka config: %w", err)
	}

	workerConfig, err := loadWorkerConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load worker config: %w", err)
	}

	return &WorkerConfig{
		Database: dbConfig,
		Kafka:    kafkaConfig,
		Worker:   workerConfig,
	}, nil
}

func loadDatabaseConfig() (*DatabaseConfig, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	maxConnections := getEnvInt("DATABASE_MAX_CONNECTIONS", 10)
	maxIdleTime := getEnvDuration("DATABASE_MAX_IDLE_TIME", 30*time.Minute)
	maxLifetime := getEnvDuration("DATABASE_MAX_LIFETIME", 1*time.Hour)
	connectTimeout := getEnvDuration("DATABASE_CONNECT_TIMEOUT", 10*time.Second)

	return &DatabaseConfig{
		URL:            url,
		MaxConnections: maxConnections,
		MaxIdleTime:    maxIdleTime,
		MaxLifetime:    maxLifetime,
		ConnectTimeout: connectTimeout,
	}, nil
}

func loadKafkaConfig() (*kafka.Config, error) {
	brokersStr := os.Getenv("KAFKA_BROKERS")
	if brokersStr == "" {
		brokersStr = "localhost:9092"
	}
	brokers := strings.Split(brokersStr, ",")

	retryMax := getEnvInt("KAFKA_RETRY_MAX", 3)
	retryBackoff := getEnvDuration("KAFKA_RETRY_BACKOFF", 100*time.Millisecond)
	flushTimeout := getEnvDuration("KAFKA_FLUSH_TIMEOUT", 10*time.Second)

	return &kafka.Config{
		Brokers:      brokers,
		RetryMax:     retryMax,
		RetryBackoff: retryBackoff,
		FlushTimeout: flushTimeout,
	}, nil
}

func loadWorkerConfig() (*OutboxWorkerConfig, error) {
	batchSize := getEnvInt("OUTBOX_BATCH_SIZE", 100)
	pollInterval := getEnvDuration("OUTBOX_POLL_INTERVAL", 5*time.Second)
	processTimeout := getEnvDuration("OUTBOX_PROCESS_TIMEOUT", 30*time.Second)
	shutdownTimeout := getEnvDuration("OUTBOX_SHUTDOWN_TIMEOUT", 30*time.Second)

	return &OutboxWorkerConfig{
		BatchSize:       batchSize,
		PollInterval:    pollInterval,
		ProcessTimeout:  processTimeout,
		ShutdownTimeout: shutdownTimeout,
	}, nil
}

func getEnvInt(key string, defaultValue int) int {
	str := os.Getenv(key)
	if str == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}

	return value
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	str := os.Getenv(key)
	if str == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(str)
	if err != nil {
		return defaultValue
	}

	return duration
}
