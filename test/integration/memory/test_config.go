package memory

import (
	"os"
	"strconv"
	"time"
)

// TestConfig holds configurable timing and threshold values for integration tests
type TestConfig struct {
	// Lock operation timing thresholds
	LockWaitThreshold   time.Duration // Threshold to detect lock waits
	LockTimeoutDuration time.Duration // Context timeout for lock operations
	AppendDelayMax      time.Duration // Maximum delay between concurrent appends
	ClearDelayMax       time.Duration // Maximum delay for clear operations
	FlushDelayMax       time.Duration // Maximum delay for flush operations

	// Health monitoring timing
	HealthCheckInterval time.Duration // Health monitoring check interval
	HealthCheckTimeout  time.Duration // Timeout for individual health checks
	HealthWaitTimeout   time.Duration // Timeout to wait for healthy status

	// Resilience testing timing
	ResilienceTimeout    time.Duration // Short timeout for resilience tests
	ResilienceRetryDelay time.Duration // Delay between retries in resilience tests

	// General test timeouts
	DefaultTestTimeout  time.Duration // Default timeout for test operations
	AsyncOperationDelay time.Duration // Delay to wait for async operations
}

// DefaultTestConfig returns the default test configuration
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		// Lock operation timing thresholds (conservative defaults)
		LockWaitThreshold:   10 * time.Millisecond, // Increased from 5ms for stability
		LockTimeoutDuration: 5 * time.Second,
		AppendDelayMax:      20 * time.Millisecond,
		ClearDelayMax:       50 * time.Millisecond,
		FlushDelayMax:       100 * time.Millisecond,

		// Health monitoring timing
		HealthCheckInterval: 100 * time.Millisecond, // Faster for testing
		HealthCheckTimeout:  2 * time.Second,
		HealthWaitTimeout:   5 * time.Second,

		// Resilience testing timing
		ResilienceTimeout:    10 * time.Millisecond,
		ResilienceRetryDelay: 2 * time.Millisecond,

		// General test timeouts
		DefaultTestTimeout:  5 * time.Second,
		AsyncOperationDelay: 100 * time.Millisecond,
	}
}

// NewTestConfigFromEnv creates a test configuration from environment variables
// This allows overriding timing values in CI/CD environments
func NewTestConfigFromEnv() *TestConfig {
	config := DefaultTestConfig()
	// Override values from environment variables if present
	if val := os.Getenv("TEST_LOCK_WAIT_THRESHOLD_MS"); val != "" {
		if ms, err := strconv.Atoi(val); err == nil {
			config.LockWaitThreshold = time.Duration(ms) * time.Millisecond
		}
	}
	if val := os.Getenv("TEST_LOCK_TIMEOUT_SECONDS"); val != "" {
		if s, err := strconv.Atoi(val); err == nil {
			config.LockTimeoutDuration = time.Duration(s) * time.Second
		}
	}
	if val := os.Getenv("TEST_HEALTH_CHECK_INTERVAL_MS"); val != "" {
		if ms, err := strconv.Atoi(val); err == nil {
			config.HealthCheckInterval = time.Duration(ms) * time.Millisecond
		}
	}
	if val := os.Getenv("TEST_DEFAULT_TIMEOUT_SECONDS"); val != "" {
		if s, err := strconv.Atoi(val); err == nil {
			config.DefaultTestTimeout = time.Duration(s) * time.Second
		}
	}
	return config
}

// GetTestConfig returns the test configuration to use
// Checks environment variables first, falls back to defaults
func GetTestConfig() *TestConfig {
	return NewTestConfigFromEnv()
}
