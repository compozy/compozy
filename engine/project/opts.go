package project

import "github.com/compozy/compozy/engine/core"

// Opts contains project-specific configuration options that control various aspects
// of workflow execution, performance tuning, and system behavior limits.
//
// **Environment Variable Override**: All options can be overridden via environment variables
// for deployment flexibility. This allows you to:
//   - Adjust settings without modifying configuration files
//   - Use different values across environments (dev, staging, prod)
//   - Implement dynamic scaling based on system resources
//
// Example configuration with custom values:
//
//	config:
//	  max_nesting_depth: 30           # Increase for complex data structures
//	  max_string_length: 52428800     # 50MB for large document processing
//	  dispatcher_heartbeat_interval: 15 # More frequent health checks
//	  async_token_counter_workers: 20   # More workers for high-volume usage
type Opts struct {
	// GlobalOpts embeds common configuration options shared across all Compozy components
	core.GlobalOpts `json:",inline" yaml:",inline" mapstructure:",squash"`

	// MaxNestingDepth limits the maximum depth of nested data structures to prevent stack overflow.
	//
	// **Default**: `20` levels
	// **Environment variable**: `MAX_NESTING_DEPTH`
	MaxNestingDepth int `json:"max_nesting_depth,omitempty" yaml:"max_nesting_depth,omitempty" mapstructure:"max_nesting_depth"`

	// MaxStringLength sets the maximum allowed length for string values in bytes.
	//
	// **Purpose**: Prevents memory exhaustion from extremely large string values such as:
	//   - Base64 encoded files
	//   - Large API responses
	//   - Accumulated text from multiple operations
	//
	// **Default**: `10,485,760` bytes (10MB)
	// **Environment variable**: `MAX_STRING_LENGTH`
	//
	// Common adjustments:
	//   - Document processing: 50MB+ for PDFs, large texts
	//   - API integrations: 1-5MB for typical JSON responses
	//   - Chat applications: 100KB-1MB for conversation history
	MaxStringLength int `json:"max_string_length,omitempty" yaml:"max_string_length,omitempty" mapstructure:"max_string_length"`

	// DispatcherHeartbeatInterval defines how often (in seconds) dispatchers send heartbeat signals.
	//
	// **Purpose**: Enables health monitoring and failure detection for distributed execution.
	// Heartbeats indicate:
	//   - Dispatcher is alive and processing
	//   - Network connectivity is maintained
	//   - System resources are available
	//
	// **Default**: `30` seconds
	// **Environment variable**: `DISPATCHER_HEARTBEAT_INTERVAL`
	//
	// Tuning considerations:
	//   - Decrease for faster failure detection (higher network overhead)
	//   - Increase for reduced network traffic (slower failure detection)
	//   - Balance with network latency and reliability requirements
	DispatcherHeartbeatInterval int `json:"dispatcher_heartbeat_interval,omitempty" yaml:"dispatcher_heartbeat_interval,omitempty" mapstructure:"dispatcher_heartbeat_interval"`

	// DispatcherHeartbeatTTL sets the time-to-live (in seconds) for dispatcher heartbeat records.
	//
	// **Purpose**: Controls retention of heartbeat data for:
	//   - Historical analysis of dispatcher health
	//   - Debugging intermittent failures
	//   - Cleanup of stale records
	//
	// **Default**: `300` seconds (5 minutes)
	// **Environment variable**: `DISPATCHER_HEARTBEAT_TTL`
	//
	// Best practices:
	//   - Set to at least 3x the heartbeat interval
	//   - Increase for better historical visibility
	//   - Consider storage capacity for high-frequency heartbeats
	DispatcherHeartbeatTTL int `json:"dispatcher_heartbeat_ttl,omitempty" yaml:"dispatcher_heartbeat_ttl,omitempty" mapstructure:"dispatcher_heartbeat_ttl"`

	// DispatcherStaleThreshold defines when (in seconds) a dispatcher is considered stale/inactive.
	//
	// **Purpose**: Triggers automatic failover and rebalancing when dispatchers become unresponsive.
	// A stale dispatcher indicates:
	//   - Process crash or hang
	//   - Network partition
	//   - Resource exhaustion
	//
	// **Default**: `120` seconds (2 minutes)
	// **Environment variable**: `DISPATCHER_STALE_THRESHOLD`
	//
	// Configuration strategy:
	//   - Set to 2-4x the heartbeat interval
	//   - Lower values for critical workflows requiring fast failover
	//   - Higher values to avoid false positives from temporary network issues
	DispatcherStaleThreshold int `json:"dispatcher_stale_threshold,omitempty" yaml:"dispatcher_stale_threshold,omitempty" mapstructure:"dispatcher_stale_threshold"`

	// MaxMessageContentLength limits the size (in bytes) of individual message content.
	//
	// **Purpose**: Prevents oversized messages from:
	//   - Overwhelming message queues
	//   - Causing out-of-memory errors
	//   - Degrading system performance
	//
	// **Default**: `10,240` bytes (10KB)
	// **Environment variable**: `MAX_MESSAGE_CONTENT_LENGTH`
	//
	// Use case examples:
	//   - Chat messages: 1-5KB typical
	//   - System notifications: 500 bytes - 2KB
	//   - Data payloads: Consider using object storage for large data
	MaxMessageContentLength int `json:"max_message_content_length,omitempty" yaml:"max_message_content_length,omitempty" mapstructure:"max_message_content_length"`

	// MaxTotalContentSize sets the maximum total size (in bytes) for all content in a single operation.
	//
	// **Purpose**: Aggregate memory protection when operations involve multiple content pieces:
	//   - Batch processing multiple messages
	//   - Aggregating results from parallel tasks
	//   - Building composite responses
	//
	// **Default**: `102,400` bytes (100KB)
	// **Environment variable**: `MAX_TOTAL_CONTENT_SIZE`
	//
	// Scaling guidelines:
	//   - Should be 5-10x MaxMessageContentLength for batch operations
	//   - Consider available memory and concurrent operations
	//   - Monitor actual usage patterns in production
	MaxTotalContentSize int `json:"max_total_content_size,omitempty" yaml:"max_total_content_size,omitempty" mapstructure:"max_total_content_size"`

	// AsyncTokenCounterWorkers specifies the number of worker goroutines for asynchronous token counting.
	//
	// **Purpose**: Parallel processing of LLM token usage for:
	//   - Accurate billing and cost tracking
	//   - Usage analytics and optimization
	//   - Rate limit management
	//
	// **Default**: `10` workers
	// **Environment variable**: `ASYNC_TOKEN_COUNTER_WORKERS`
	//
	// Performance tuning:
	//   - Increase for high-volume LLM usage (100+ requests/second)
	//   - Decrease for low-volume or development environments
	//   - Monitor CPU usage and queue depth
	AsyncTokenCounterWorkers int `json:"async_token_counter_workers,omitempty" yaml:"async_token_counter_workers,omitempty" mapstructure:"async_token_counter_workers"`

	// AsyncTokenCounterBufferSize sets the buffer size for the async token counter queue.
	//
	// **Purpose**: Buffers token counting requests to:
	//   - Smooth out traffic spikes
	//   - Prevent blocking on LLM operations
	//   - Optimize memory usage
	//
	// **Default**: `1,000` items
	// **Environment variable**: `ASYNC_TOKEN_COUNTER_BUFFER_SIZE`
	//
	// Capacity planning:
	//   - Size based on peak requests per second
	//   - Larger buffers handle bursts better but use more memory
	//   - Monitor queue saturation and adjust accordingly
	AsyncTokenCounterBufferSize int `json:"async_token_counter_buffer_size,omitempty" yaml:"async_token_counter_buffer_size,omitempty" mapstructure:"async_token_counter_buffer_size"`
}
