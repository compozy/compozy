package cache

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config defines the Redis cache configuration for Compozy's distributed caching layer.
//
// **Caching** in Compozy accelerates workflow execution by storing:
//   - **LLM responses**: Reduce API costs by caching model outputs
//   - **Tool results**: Cache expensive computation results
//   - **Workflow state**: Enable distributed workflow coordination
//   - **Session data**: Maintain conversation context across requests
//
// The cache layer is essential for building scalable, cost-effective AI applications
// that can handle high throughput while maintaining low latency.
//
// ## Basic Cache Configuration
//
//	cache:
//	  url: redis://localhost:6379/0
//
// ## Advanced Cache Configuration
//
//	cache:
//	  host: redis.example.com
//	  port: 6380
//	  password: "{{ .env.REDIS_PASSWORD }}"
//	  db: 1
//	  pool_size: 20
//	  tls_enabled: true
//	  dial_timeout: 5s
//	  read_timeout: 3s
//	  write_timeout: 3s
//	  max_retries: 3
//	  min_retry_backoff: 8ms
//	  max_retry_backoff: 512ms
//
// ## High-Performance Configuration
//
//	cache:
//	  url: redis://cache-cluster:6379/0
//	  pool_size: 100
//	  min_idle_conns: 10
//	  max_idle_conns: 50
//	  pool_timeout: 10s
//	  notification_buffer_size: 1000
type Config struct {
	// URL provides a complete Redis connection string.
	//
	// Format: `redis://[username][:password]@host[:port]/[db]`
	//
	// **Examples**:
	//   - `redis://localhost:6379/0` - Local Redis, database 0
	//   - `redis://:password@redis.example.com:6379/1` - With password
	//   - `rediss://cache.example.com:6380/0` - TLS connection
	//
	// When URL is provided, it takes precedence over individual
	// connection parameters (host, port, password, db).
	URL string `json:"url,omitempty" yaml:"url,omitempty" mapstructure:"url"`

	// Host specifies the Redis server hostname or IP address.
	//
	// Used when URL is not provided. Can be:
	//   - Hostname: `redis.example.com`
	//   - IP address: `192.168.1.100`
	//   - Unix socket: `/var/run/redis/redis.sock`
	//
	// **Default**: `localhost`
	Host string `json:"host,omitempty" yaml:"host,omitempty" mapstructure:"host"`

	// Port specifies the Redis server port.
	//
	// **Default**: `6379` (standard Redis port)
	//
	// Common ports:
	//   - `6379` - Default Redis port
	//   - `6380` - Common alternative
	//   - `26379` - Redis Sentinel
	Port string `json:"port,omitempty" yaml:"port,omitempty" mapstructure:"port"`

	// Password for Redis authentication.
	//
	// - Required when Redis is configured with `requirepass`.
	// - Supports template variables: `"{{ .env.REDIS_PASSWORD }}"`
	//
	// **Security**: Store passwords in environment variables,
	// never commit them to version control.
	Password string `json:"password,omitempty" yaml:"password,omitempty" mapstructure:"password"`

	// DB selects the Redis database number.
	//
	// Redis supports multiple logical databases (0-15 by default).
	// Different databases can be used to separate:
	//   - Environments (dev=0, staging=1, prod=2)
	//   - Data types (cache=0, sessions=1, queues=2)
	//   - Applications sharing the same Redis instance
	//
	// **Default**: `0`
	DB int `json:"db,omitempty" yaml:"db,omitempty" mapstructure:"db"`

	// PoolSize sets the maximum number of connections in the pool.
	//
	// Determines how many concurrent Redis operations can execute.
	// Set based on:
	//   - Expected concurrent workflows
	//   - Redis server connection limits
	//   - Available system resources
	//
	// **Default**: `10 * runtime.NumCPU()`
	PoolSize int `json:"pool_size,omitempty" yaml:"pool_size,omitempty" mapstructure:"pool_size"`

	// TLSEnabled activates TLS/SSL encryption for Redis connections.
	//
	// Enable for:
	//   - Cloud Redis instances (AWS ElastiCache, Azure Cache)
	//   - Redis 6+ with TLS support
	//   - Connections over untrusted networks
	//
	// **Default**: `false`
	TLSEnabled bool `json:"tls_enabled,omitempty" yaml:"tls_enabled,omitempty" mapstructure:"tls_enabled"`

	// TLSConfig provides custom TLS configuration.
	//
	// This field is populated programmatically and cannot be set via YAML.
	// Use TLSEnabled to activate TLS with default settings.
	TLSConfig *tls.Config `json:"-" yaml:"-" mapstructure:"-"` // Not serializable

	// DialTimeout limits the time to establish a connection.
	//
	// Prevents indefinite hanging when Redis is unreachable.
	// Increase for high-latency networks or overloaded servers.
	//
	// **Default**: `5s`
	DialTimeout time.Duration `json:"dial_timeout,omitempty" yaml:"dial_timeout,omitempty" mapstructure:"dial_timeout"`

	// ReadTimeout limits the time to read a response.
	//
	// Applies to each Redis command execution. Set based on:
	//   - Slowest expected Redis operation
	//   - Network latency
	//   - Redis server performance
	//
	// **Default**: `3s`
	ReadTimeout time.Duration `json:"read_timeout,omitempty" yaml:"read_timeout,omitempty" mapstructure:"read_timeout"`

	// WriteTimeout limits the time to write a request.
	//
	// Usually shorter than ReadTimeout since writes are typically fast.
	// Increase for large value writes or slow networks.
	//
	// **Default**: `3s`
	WriteTimeout time.Duration `json:"write_timeout,omitempty" yaml:"write_timeout,omitempty" mapstructure:"write_timeout"`

	// MaxRetries sets the maximum retry attempts for failed commands.
	//
	// Retries help with transient failures like:
	//   - Network hiccups
	//   - Redis server restarts
	//   - Temporary overload
	//
	// Set to 0 to disable retries.
	//
	// **Default**: `3`
	MaxRetries int `json:"max_retries,omitempty" yaml:"max_retries,omitempty" mapstructure:"max_retries"`

	// MinRetryBackoff sets the minimum wait time between retries.
	//
	// Uses exponential backoff: wait time doubles after each retry,
	// capped at MaxRetryBackoff.
	//
	// **Default**: `8ms`
	MinRetryBackoff time.Duration `json:"min_retry_backoff,omitempty" yaml:"min_retry_backoff,omitempty" mapstructure:"min_retry_backoff"`

	// MaxRetryBackoff sets the maximum wait time between retries.
	//
	// Prevents excessive wait times during extended outages.
	//
	// **Default**: `512ms`
	MaxRetryBackoff time.Duration `json:"max_retry_backoff,omitempty" yaml:"max_retry_backoff,omitempty" mapstructure:"max_retry_backoff"`

	// MaxIdleConns limits the number of idle connections in the pool.
	//
	// Idle connections are kept open for reuse, reducing connection
	// overhead. Set based on:
	//   - Typical concurrent load
	//   - Memory constraints
	//   - Redis connection limits
	//
	// **Default**: Same as PoolSize
	MaxIdleConns int `json:"max_idle_conns,omitempty" yaml:"max_idle_conns,omitempty" mapstructure:"max_idle_conns"`

	// MinIdleConns maintains a minimum number of idle connections.
	//
	// Ensures connections are pre-established for better latency.
	// Useful for:
	//   - Consistent performance requirements
	//   - Predictable load patterns
	//   - Reducing cold start latency
	//
	// **Default**: `0`
	MinIdleConns int `json:"min_idle_conns,omitempty" yaml:"min_idle_conns,omitempty" mapstructure:"min_idle_conns"`

	// PoolTimeout limits the time to wait for a free connection.
	//
	// When all connections are busy, new requests wait up to this
	// duration before failing. Prevents indefinite blocking.
	//
	// **Default**: `ReadTimeout + 1s`
	PoolTimeout time.Duration `json:"pool_timeout,omitempty" yaml:"pool_timeout,omitempty" mapstructure:"pool_timeout"`

	// PingTimeout limits the time for health check pings.
	//
	// Used to verify connection health before reuse.
	// Should be shorter than command timeouts.
	//
	// **Default**: `1s`
	PingTimeout time.Duration `json:"ping_timeout,omitempty" yaml:"ping_timeout,omitempty" mapstructure:"ping_timeout"`

	// NotificationBufferSize sets the buffer size for pub/sub notifications.
	//
	// Determines how many messages can be buffered before blocking.
	// Increase for:
	//   - High-frequency pub/sub usage
	//   - Bursty message patterns
	//   - Slow message consumers
	//
	// **Default**: `100`
	NotificationBufferSize int `json:"notification_buffer_size,omitempty" yaml:"notification_buffer_size,omitempty" mapstructure:"notification_buffer_size"`
}

// Validate checks if the cache configuration has valid values
func (c *Config) Validate() error {
	if err := c.validateBasicConfig(); err != nil {
		return err
	}
	if err := c.validatePoolConfig(); err != nil {
		return err
	}
	if err := c.validateTimeoutConfig(); err != nil {
		return err
	}
	if err := c.validateRetryConfig(); err != nil {
		return err
	}
	return nil
}

// validateBasicConfig validates basic configuration fields
func (c *Config) validateBasicConfig() error {
	if c.DB < 0 {
		return fmt.Errorf("cache DB index cannot be negative: got %d", c.DB)
	}
	if c.URL == "" && strings.TrimSpace(c.Host) == "" {
		return errors.New("cache host cannot be empty when URL is not provided")
	}
	if c.Port != "" {
		if port, err := strconv.Atoi(c.Port); err != nil {
			return fmt.Errorf("invalid cache port: %s", c.Port)
		} else if port <= 0 || port > 65535 {
			return fmt.Errorf("cache port must be between 1 and 65535: got %d", port)
		}
	}
	return nil
}

// validatePoolConfig validates pool-related configuration
func (c *Config) validatePoolConfig() error {
	if c.PoolSize < 0 {
		return fmt.Errorf("cache pool size cannot be negative: got %d", c.PoolSize)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("cache max retries cannot be negative: got %d", c.MaxRetries)
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("cache max idle connections cannot be negative: got %d", c.MaxIdleConns)
	}
	if c.MinIdleConns < 0 {
		return fmt.Errorf("cache min idle connections cannot be negative: got %d", c.MinIdleConns)
	}
	if c.MinIdleConns > c.MaxIdleConns && c.MaxIdleConns > 0 {
		return fmt.Errorf(
			"cache min idle connections (%d) cannot be greater than max idle connections (%d)",
			c.MinIdleConns,
			c.MaxIdleConns,
		)
	}
	if c.NotificationBufferSize < 0 {
		return fmt.Errorf("cache notification buffer size cannot be negative: got %d", c.NotificationBufferSize)
	}
	return nil
}

// validateTimeoutConfig validates timeout-related configuration
func (c *Config) validateTimeoutConfig() error {
	if c.DialTimeout < 0 {
		return fmt.Errorf("cache dial timeout cannot be negative: got %v", c.DialTimeout)
	}
	if c.ReadTimeout < 0 {
		return fmt.Errorf("cache read timeout cannot be negative: got %v", c.ReadTimeout)
	}
	if c.WriteTimeout < 0 {
		return fmt.Errorf("cache write timeout cannot be negative: got %v", c.WriteTimeout)
	}
	if c.PoolTimeout < 0 {
		return fmt.Errorf("cache pool timeout cannot be negative: got %v", c.PoolTimeout)
	}
	if c.PingTimeout < 0 {
		return fmt.Errorf("cache ping timeout cannot be negative: got %v", c.PingTimeout)
	}
	return nil
}

// validateRetryConfig validates retry backoff configuration
func (c *Config) validateRetryConfig() error {
	if c.MinRetryBackoff < 0 {
		return fmt.Errorf("cache min retry backoff cannot be negative: got %v", c.MinRetryBackoff)
	}
	if c.MaxRetryBackoff < 0 {
		return fmt.Errorf("cache max retry backoff cannot be negative: got %v", c.MaxRetryBackoff)
	}
	if c.MinRetryBackoff > c.MaxRetryBackoff && c.MaxRetryBackoff > 0 {
		return fmt.Errorf(
			"cache min retry backoff (%v) cannot be greater than max retry backoff (%v)",
			c.MinRetryBackoff,
			c.MaxRetryBackoff,
		)
	}
	return nil
}
