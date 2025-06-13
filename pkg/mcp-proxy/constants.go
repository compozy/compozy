package mcpproxy

import "time"

// Connection timeouts and delays
const (
	// DefaultConnectTimeout is the default timeout for establishing MCP connections
	DefaultConnectTimeout = 10 * time.Second

	// QuickConnectTimeout is used for immediate connection attempts before falling back to async
	QuickConnectTimeout = 3 * time.Second

	// AdminHealthCheckTimeout is the timeout for admin API health checks
	AdminHealthCheckTimeout = 5 * time.Second

	// ToolCallTimeout is the default timeout for executing tools
	ToolCallTimeout = 30 * time.Second

	// PingTimeout is the timeout for ping operations
	PingTimeout = 10 * time.Second

	// ShutdownGraceTimeout is the maximum time to wait for background operations to complete
	ShutdownGraceTimeout = 5 * time.Second
)

// Retry and reconnection settings
const (
	// DefaultMaxReconnects is the default maximum number of reconnection attempts
	DefaultMaxReconnects = 5

	// DefaultReconnectDelay is the default delay between reconnection attempts
	DefaultReconnectDelay = 5 * time.Second

	// MaxBackoffDelay is the maximum delay for exponential backoff
	MaxBackoffDelay = 60 * time.Second

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier = 1.5
)

// Concurrency limits
const (
	// MaxConcurrentResourceAdds limits concurrent resource registration operations
	MaxConcurrentResourceAdds = 5

	// DefaultHealthCheckParallelism limits concurrent health checks
	DefaultHealthCheckParallelism = 8

	// DefaultMaxConnections is the default maximum number of concurrent client connections
	DefaultMaxConnections = 100

	// RedisScanBatchSize is the batch size for Redis SCAN operations
	RedisScanBatchSize = 100
)

// Intervals and monitoring
const (
	// DefaultHealthCheckInterval is how often to perform health checks
	DefaultHealthCheckInterval = 30 * time.Second

	// PingInterval is how often to ping connected clients
	PingInterval = 30 * time.Second

	// ServerStartupCheckDelay is how long to wait before checking if server started successfully
	ServerStartupCheckDelay = 100 * time.Millisecond
)

// Security and validation
const (
	// MinTokenLength is the minimum required length for admin tokens
	MinTokenLength = 16

	// BearerTokenPrefix is the expected prefix for Bearer tokens
	BearerTokenPrefix = "Bearer "
)

// Default values for MCP definitions
const (
	// DefaultRequestTimeout is the default timeout for MCP requests
	DefaultRequestTimeout = 30 * time.Second

	// DefaultHTTPTimeout is the default HTTP timeout for HTTP-based transports
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultHealthCheckIntervalForMCP is the default health check interval for individual MCPs
	DefaultHealthCheckIntervalForMCP = 30 * time.Second
)
