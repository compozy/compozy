package runtime

import (
	"os"
	"time"
)

// Config holds configuration for the RuntimeManager
type Config struct {
	BackoffInitialInterval time.Duration
	BackoffMaxInterval     time.Duration
	BackoffMaxElapsedTime  time.Duration
	WorkerFilePerm         os.FileMode
	DenoPermissions        []string
	ToolExecutionTimeout   time.Duration
	DenoNoCheck            bool
}

// Option is a function that configures the RuntimeManager
type Option func(*Config)

// WithBackoffInitialInterval sets the initial backoff interval
func WithBackoffInitialInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.BackoffInitialInterval = interval
	}
}

// WithBackoffMaxInterval sets the maximum backoff interval
func WithBackoffMaxInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.BackoffMaxInterval = interval
	}
}

// WithBackoffMaxElapsedTime sets the maximum elapsed time for backoff
func WithBackoffMaxElapsedTime(elapsed time.Duration) Option {
	return func(c *Config) {
		c.BackoffMaxElapsedTime = elapsed
	}
}

// WithWorkerFilePerm sets the file permissions for worker files
func WithWorkerFilePerm(perm os.FileMode) Option {
	return func(c *Config) {
		c.WorkerFilePerm = perm
	}
}

// WithDenoPermissions sets the Deno permissions
func WithDenoPermissions(permissions []string) Option {
	return func(c *Config) {
		c.DenoPermissions = permissions
	}
}

// WithToolExecutionTimeout sets the tool execution timeout
func WithToolExecutionTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.ToolExecutionTimeout = timeout
	}
}

// WithDenoNoCheck sets whether to use --no-check flag
func WithDenoNoCheck(noCheck bool) Option {
	return func(c *Config) {
		c.DenoNoCheck = noCheck
	}
}

// WithTestConfig applies test-specific configuration
func WithTestConfig() Option {
	return func(c *Config) {
		testConfig := TestConfig()
		*c = *testConfig
	}
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		BackoffInitialInterval: 100 * time.Millisecond,
		BackoffMaxInterval:     5 * time.Second,
		BackoffMaxElapsedTime:  30 * time.Second,
		WorkerFilePerm:         0700,
		DenoPermissions: []string{
			"--allow-net",
			"--allow-env",
		},
		ToolExecutionTimeout: 60 * time.Second,
		DenoNoCheck:          true, // Default to true for performance
	}
}

func TestConfig() *Config {
	return &Config{
		BackoffInitialInterval: 10 * time.Millisecond,
		BackoffMaxInterval:     100 * time.Millisecond,
		BackoffMaxElapsedTime:  1 * time.Second, // Much shorter for tests
		WorkerFilePerm:         0700,
		DenoPermissions: []string{
			"--allow-read",
			"--allow-net",
			"--allow-env",
		},
		ToolExecutionTimeout: 5 * time.Second, // Shorter timeout for tests
		DenoNoCheck:          true,
	}
}
