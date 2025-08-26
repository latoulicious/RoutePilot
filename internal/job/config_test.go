package job

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromEnv_Success(t *testing.T) {
	// Setup environment variables
	envVars := map[string]string{
		"DATABASE_URL":             "postgres://user:pass@localhost:5432/testdb",
		"DATABASE_MAX_CONNECTIONS": "20",
		"DATABASE_MAX_IDLE_TIME":   "15m",
		"DATABASE_MAX_LIFETIME":    "2h",
		"DATABASE_CONNECT_TIMEOUT": "5s",
		"KAFKA_BROKERS":            "broker1:9092,broker2:9092",
		"KAFKA_RETRY_MAX":          "5",
		"KAFKA_RETRY_BACKOFF":      "200ms",
		"KAFKA_FLUSH_TIMEOUT":      "15s",
		"OUTBOX_BATCH_SIZE":        "50",
		"OUTBOX_POLL_INTERVAL":     "10s",
		"OUTBOX_PROCESS_TIMEOUT":   "45s",
		"OUTBOX_SHUTDOWN_TIMEOUT":  "60s",
	}

	// Set environment variables
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	// Execute
	config, err := LoadConfigFromEnv()

	// Assert
	require.NoError(t, err)
	require.NotNil(t, config)

	// Check database config
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", config.Database.URL)
	assert.Equal(t, 20, config.Database.MaxConnections)
	assert.Equal(t, 15*time.Minute, config.Database.MaxIdleTime)
	assert.Equal(t, 2*time.Hour, config.Database.MaxLifetime)
	assert.Equal(t, 5*time.Second, config.Database.ConnectTimeout)

	// Check Kafka config
	assert.Equal(t, []string{"broker1:9092", "broker2:9092"}, config.Kafka.Brokers)
	assert.Equal(t, 5, config.Kafka.RetryMax)
	assert.Equal(t, 200*time.Millisecond, config.Kafka.RetryBackoff)
	assert.Equal(t, 15*time.Second, config.Kafka.FlushTimeout)

	// Check worker config
	assert.Equal(t, 50, config.Worker.BatchSize)
	assert.Equal(t, 10*time.Second, config.Worker.PollInterval)
	assert.Equal(t, 45*time.Second, config.Worker.ProcessTimeout)
	assert.Equal(t, 60*time.Second, config.Worker.ShutdownTimeout)
}

func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	// Setup minimal environment (only required vars)
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
	defer os.Unsetenv("DATABASE_URL")

	// Clear other environment variables to test defaults
	envVarsToUnset := []string{
		"DATABASE_MAX_CONNECTIONS",
		"DATABASE_MAX_IDLE_TIME",
		"DATABASE_MAX_LIFETIME",
		"DATABASE_CONNECT_TIMEOUT",
		"KAFKA_BROKERS",
		"KAFKA_RETRY_MAX",
		"KAFKA_RETRY_BACKOFF",
		"KAFKA_FLUSH_TIMEOUT",
		"OUTBOX_BATCH_SIZE",
		"OUTBOX_POLL_INTERVAL",
		"OUTBOX_PROCESS_TIMEOUT",
		"OUTBOX_SHUTDOWN_TIMEOUT",
	}
	for _, key := range envVarsToUnset {
		os.Unsetenv(key)
	}

	// Execute
	config, err := LoadConfigFromEnv()

	// Assert
	require.NoError(t, err)
	require.NotNil(t, config)

	// Check database defaults
	assert.Equal(t, "postgres://user:pass@localhost:5432/testdb", config.Database.URL)
	assert.Equal(t, 10, config.Database.MaxConnections)
	assert.Equal(t, 30*time.Minute, config.Database.MaxIdleTime)
	assert.Equal(t, 1*time.Hour, config.Database.MaxLifetime)
	assert.Equal(t, 10*time.Second, config.Database.ConnectTimeout)

	// Check Kafka defaults
	assert.Equal(t, []string{"localhost:9092"}, config.Kafka.Brokers)
	assert.Equal(t, 3, config.Kafka.RetryMax)
	assert.Equal(t, 100*time.Millisecond, config.Kafka.RetryBackoff)
	assert.Equal(t, 10*time.Second, config.Kafka.FlushTimeout)

	// Check worker defaults
	assert.Equal(t, 100, config.Worker.BatchSize)
	assert.Equal(t, 5*time.Second, config.Worker.PollInterval)
	assert.Equal(t, 30*time.Second, config.Worker.ProcessTimeout)
	assert.Equal(t, 30*time.Second, config.Worker.ShutdownTimeout)
}

func TestLoadConfigFromEnv_MissingDatabaseURL(t *testing.T) {
	// Ensure DATABASE_URL is not set
	os.Unsetenv("DATABASE_URL")

	// Execute
	config, err := LoadConfigFromEnv()

	// Assert
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "DATABASE_URL environment variable is required")
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "valid integer",
			envValue:     "42",
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "empty string",
			envValue:     "",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "invalid integer",
			envValue:     "not-a-number",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "negative integer",
			envValue:     "-5",
			defaultValue: 10,
			expected:     -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_INT_VAR"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
			} else {
				os.Unsetenv(key)
			}
			defer os.Unsetenv(key)

			result := getEnvInt(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{
			name:         "valid duration",
			envValue:     "5m30s",
			defaultValue: 1 * time.Minute,
			expected:     5*time.Minute + 30*time.Second,
		},
		{
			name:         "empty string",
			envValue:     "",
			defaultValue: 1 * time.Minute,
			expected:     1 * time.Minute,
		},
		{
			name:         "invalid duration",
			envValue:     "not-a-duration",
			defaultValue: 1 * time.Minute,
			expected:     1 * time.Minute,
		},
		{
			name:         "seconds only",
			envValue:     "30s",
			defaultValue: 1 * time.Minute,
			expected:     30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_DURATION_VAR"
			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
			} else {
				os.Unsetenv(key)
			}
			defer os.Unsetenv(key)

			result := getEnvDuration(key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}
