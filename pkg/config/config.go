package config

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/compozy/compozy/pkg/config/definition"
)

// Config represents the complete configuration for the Compozy system.
//
// **Application Configuration** controls the runtime behavior of the Compozy server and services.
// This configuration is typically provided through environment variables, configuration files,
// or command-line flags, and defines how the system operates at runtime.
//
// Configuration can be overridden at multiple levels:
//   - **System defaults**: Built-in safe defaults for all settings
//   - **Configuration file**: YAML file with environment-specific settings
//   - **Environment variables**: Override specific values for deployment
//   - **Command-line flags**: Highest precedence for runtime overrides
//
// ## Example Configuration
//
//	server:
//	  host: 0.0.0.0
//	  port: 8080
//	  cors_enabled: true
//	database:
//	  host: localhost
//	  port: 5432
//	  name: compozy
//	temporal:
//	  host_port: localhost:7233
//	  namespace: default
//	runtime:
//	  environment: development
//	  log_level: info
type Config struct {
	// Server configures the HTTP API server settings.
	//
	// $ref: schema://application#server
	Server ServerConfig `koanf:"server" json:"server" yaml:"server" mapstructure:"server" validate:"required"`

	// Database configures the PostgreSQL database connection.
	//
	// $ref: schema://application#database
	Database DatabaseConfig `koanf:"database" json:"database" yaml:"database" mapstructure:"database" validate:"required"`

	// Temporal configures the workflow engine connection.
	//
	// $ref: schema://application#temporal
	Temporal TemporalConfig `koanf:"temporal" json:"temporal" yaml:"temporal" mapstructure:"temporal" validate:"required"`

	// Runtime configures system runtime behavior and performance.
	//
	// $ref: schema://application#runtime
	Runtime RuntimeConfig `koanf:"runtime" json:"runtime" yaml:"runtime" mapstructure:"runtime" validate:"required"`

	// Limits defines system resource limits and constraints.
	//
	// $ref: schema://application#limits
	Limits LimitsConfig `koanf:"limits" json:"limits" yaml:"limits" mapstructure:"limits" validate:"required"`

	// RateLimit configures API rate limiting.
	//
	// $ref: schema://application#ratelimit
	RateLimit RateLimitConfig `koanf:"ratelimit" json:"ratelimit" yaml:"ratelimit" mapstructure:"ratelimit"`

	// Memory configures the memory service for agent conversations.
	//
	// $ref: schema://application#memory
	Memory MemoryConfig `koanf:"memory" json:"memory" yaml:"memory" mapstructure:"memory"`

	// LLM configures the LLM proxy service.
	//
	// $ref: schema://application#llm
	LLM LLMConfig `koanf:"llm" json:"llm" yaml:"llm" mapstructure:"llm"`

	// CLI configures command-line interface behavior.
	//
	// $ref: schema://application#cli
	CLI CLIConfig `koanf:"cli" json:"cli" yaml:"cli" mapstructure:"cli"`

	// Redis configures the Redis connection for all services.
	//
	// $ref: schema://application#redis
	Redis RedisConfig `koanf:"redis" json:"redis" yaml:"redis" mapstructure:"redis" validate:"required"`

	// Cache configures caching behavior and performance settings.
	//
	// $ref: schema://application#cache
	Cache CacheConfig `koanf:"cache" json:"cache" yaml:"cache" mapstructure:"cache"`

	// Worker configures Temporal worker settings.
	//
	// $ref: schema://application#worker
	Worker WorkerConfig `koanf:"worker" json:"worker" yaml:"worker" mapstructure:"worker" validate:"required"`

	// MCPProxy configures the MCP proxy server.
	//
	// $ref: schema://application#mcpproxy
	MCPProxy MCPProxyConfig `koanf:"mcp_proxy" json:"mcp_proxy" yaml:"mcp_proxy" mapstructure:"mcp_proxy" validate:"required"`
}

// ServerConfig contains HTTP server configuration.
//
// **HTTP Server Settings** control how the Compozy API server listens for and handles requests.
// These settings affect performance, security, and client compatibility.
//
// ## Example Configuration
//
//	server:
//	  host: 0.0.0.0
//	  port: 8080
//	  timeout: 30s
//	  cors_enabled: true
//	  cors:
//	    allowed_origins:
//	      - http://localhost:3000
//	      - https://app.example.com
//	    allow_credentials: true
//	  auth:
//	    enabled: true
//	    workflow_exceptions:
//	      - public-workflow-id
type ServerConfig struct {
	// Host specifies the network interface to bind the server to.
	//
	// Common values:
	//   - `"0.0.0.0"`: Listen on all interfaces (default)
	//   - `"127.0.0.1"` or `"localhost"`: Local access only
	//   - Specific IP: Bind to a specific network interface
	Host string `koanf:"host" json:"host" yaml:"host" mapstructure:"host" validate:"required" env:"SERVER_HOST"`

	// Port specifies the TCP port for the HTTP server.
	//
	// Valid range: 1-65535. Common ports:
	//   - `8080`: Default development port
	//   - `80`: Standard HTTP (requires privileges)
	//   - `443`: Standard HTTPS (requires privileges)
	Port int `koanf:"port" json:"port" yaml:"port" mapstructure:"port" validate:"min=1,max=65535" env:"SERVER_PORT"`

	// CORSEnabled enables Cross-Origin Resource Sharing headers.
	//
	// Set to `true` when the API is accessed from web browsers on different origins.
	CORSEnabled bool `koanf:"cors_enabled" json:"cors_enabled" yaml:"cors_enabled" mapstructure:"cors_enabled" env:"SERVER_CORS_ENABLED"`

	// CORS configures Cross-Origin Resource Sharing policies.
	//
	// Only applies when CORSEnabled is true.
	CORS CORSConfig `koanf:"cors" json:"cors" yaml:"cors" mapstructure:"cors"`

	// Timeout sets the maximum duration for processing requests.
	//
	// Applies to all HTTP operations including request reading, processing, and response writing.
	// Default: 30s. Increase for long-running operations.
	Timeout time.Duration `koanf:"timeout" json:"timeout" yaml:"timeout" mapstructure:"timeout" env:"SERVER_TIMEOUT"`

	// Auth configures API authentication settings.
	Auth AuthConfig `koanf:"auth" json:"auth" yaml:"auth" mapstructure:"auth"`
}

// CORSConfig contains CORS configuration.
type CORSConfig struct {
	AllowedOrigins   []string `koanf:"allowed_origins"   json:"allowed_origins"   yaml:"allowed_origins"   mapstructure:"allowed_origins"   env:"SERVER_CORS_ALLOWED_ORIGINS"`
	AllowCredentials bool     `koanf:"allow_credentials" json:"allow_credentials" yaml:"allow_credentials" mapstructure:"allow_credentials" env:"SERVER_CORS_ALLOW_CREDENTIALS"`
	MaxAge           int      `koanf:"max_age"           json:"max_age"           yaml:"max_age"           mapstructure:"max_age"           env:"SERVER_CORS_MAX_AGE"`
}

// AuthConfig contains authentication configuration.
type AuthConfig struct {
	Enabled            bool            `koanf:"enabled"             json:"enabled"             yaml:"enabled"             mapstructure:"enabled"             env:"SERVER_AUTH_ENABLED"`
	WorkflowExceptions []string        `koanf:"workflow_exceptions" json:"workflow_exceptions" yaml:"workflow_exceptions" mapstructure:"workflow_exceptions" env:"SERVER_AUTH_WORKFLOW_EXCEPTIONS" validate:"dive,workflow_id"`
	AdminKey           SensitiveString `koanf:"admin_key"           json:"admin_key"           yaml:"admin_key"           mapstructure:"admin_key"           env:"SERVER_AUTH_ADMIN_KEY"           validate:"omitempty,min=16" sensitive:"true"`
}

// DatabaseConfig contains database connection configuration.
//
// **PostgreSQL Database Configuration** defines how Compozy connects to its primary data store.
// The database stores workflows, agents, execution history, and system state.
//
// ## Connection Methods
//
//  1. **Connection String** (preferred for production):
//     ```yaml
//     database:
//     conn_string: "postgres://user:pass@host:5432/dbname?sslmode=require"
//     ```
//
//  2. **Individual Parameters** (development/testing):
//     ```yaml
//     database:
//     host: localhost
//     port: 5432
//     user: compozy
//     password: secret
//     name: compozy_db
//     ssl_mode: prefer
//     ```
type DatabaseConfig struct {
	// ConnString provides a complete PostgreSQL connection URL.
	//
	// Format: `postgres://user:password@host:port/database?sslmode=mode`
	// Takes precedence over individual connection parameters.
	ConnString string `koanf:"conn_string" json:"conn_string" yaml:"conn_string" mapstructure:"conn_string" env:"DB_CONN_STRING"`

	// Host specifies the database server hostname or IP address.
	//
	// Default: "localhost"
	Host string `koanf:"host" json:"host" yaml:"host" mapstructure:"host" env:"DB_HOST"`

	// Port specifies the database server port.
	//
	// Default: "5432" (PostgreSQL default)
	Port string `koanf:"port" json:"port" yaml:"port" mapstructure:"port" env:"DB_PORT"`

	// User specifies the database username for authentication.
	User string `koanf:"user" json:"user" yaml:"user" mapstructure:"user" env:"DB_USER"`

	// Password specifies the database password for authentication.
	//
	// **Security**: Use environment variables in production.
	Password string `koanf:"password" json:"password" yaml:"password" mapstructure:"password" env:"DB_PASSWORD"`

	// DBName specifies the database name to connect to.
	//
	// Default: "compozy"
	DBName string `koanf:"name" json:"name" yaml:"name" mapstructure:"name" env:"DB_NAME"`

	// SSLMode configures SSL/TLS connection security.
	//
	// Options:
	//   - `"disable"`: No SSL (development only)
	//   - `"prefer"`: Try SSL, fallback to non-SSL
	//   - `"require"`: SSL required (recommended)
	//   - `"verify-ca"`: SSL with CA verification
	//   - `"verify-full"`: SSL with full verification
	SSLMode string `koanf:"ssl_mode" json:"ssl_mode" yaml:"ssl_mode" mapstructure:"ssl_mode" env:"DB_SSL_MODE"`

	// AutoMigrate enables automatic database migrations on startup.
	//
	// When enabled, the system will automatically apply any pending database
	// migrations when establishing a database connection. This eliminates
	// the need for manual migration commands.
	//
	// Default: true
	AutoMigrate bool `koanf:"auto_migrate" json:"auto_migrate" yaml:"auto_migrate" mapstructure:"auto_migrate" env:"DB_AUTO_MIGRATE"`
}

// TemporalConfig contains Temporal workflow engine configuration.
//
// **Temporal Workflow Engine** provides durable execution, retries, and orchestration
// for Compozy workflows. Temporal ensures workflows complete reliably even across
// failures, restarts, and long-running operations.
//
// ## Example Configuration
//
//	temporal:
//	  host_port: localhost:7233
//	  namespace: default
//	  task_queue: compozy-tasks
type TemporalConfig struct {
	// HostPort specifies the Temporal server endpoint.
	//
	// Format: `host:port`
	// Default: "localhost:7233"
	HostPort string `koanf:"host_port" env:"TEMPORAL_HOST_PORT" json:"host_port" yaml:"host_port" mapstructure:"host_port"`

	// Namespace isolates workflows within Temporal.
	//
	// Use different namespaces for:
	//   - Environment separation (dev, staging, prod)
	//   - Multi-tenant deployments
	//   - Workflow versioning
	// Default: "default"
	Namespace string `koanf:"namespace" env:"TEMPORAL_NAMESPACE" json:"namespace" yaml:"namespace" mapstructure:"namespace"`

	// TaskQueue identifies the queue for workflow tasks.
	//
	// Workers poll this queue for tasks to execute.
	// Use different queues for:
	//   - Workflow type separation
	//   - Priority-based routing
	//   - Resource isolation
	// Default: "compozy-tasks"
	TaskQueue string `koanf:"task_queue" env:"TEMPORAL_TASK_QUEUE" json:"task_queue" yaml:"task_queue" mapstructure:"task_queue"`
}

// RuntimeConfig contains runtime behavior configuration.
//
// **System Runtime Configuration** controls execution behavior, performance tuning,
// and operational parameters. These settings affect how workflows execute,
// how the system handles load, and how it reports operational status.
//
// ## Example Configuration
//
//	runtime:
//	  environment: production
//	  log_level: info
//	  dispatcher_heartbeat_interval: 5s
//	  async_token_counter_workers: 10
//	  tool_execution_timeout: 30s
type RuntimeConfig struct {
	// Environment specifies the deployment environment.
	//
	// Affects:
	//   - Error verbosity and stack traces
	//   - Performance optimizations
	//   - Debug endpoints availability
	//   - Default timeouts and limits
	//
	// Values: "development", "staging", "production"
	Environment string `koanf:"environment" validate:"oneof=development staging production" env:"RUNTIME_ENVIRONMENT" json:"environment" yaml:"environment" mapstructure:"environment"`

	// LogLevel controls logging verbosity.
	//
	// Levels (least to most verbose):
	//   - `"error"`: Critical errors only
	//   - `"warn"`: Warnings and errors
	//   - `"info"`: General operational info (default)
	//   - `"debug"`: Detailed debugging information
	LogLevel string `koanf:"log_level" validate:"oneof=debug info warn error" env:"RUNTIME_LOG_LEVEL" json:"log_level" yaml:"log_level" mapstructure:"log_level"`

	// DispatcherHeartbeatInterval sets how often dispatchers report health.
	//
	// Lower values provide faster failure detection but increase load.
	// Default: 30s
	DispatcherHeartbeatInterval time.Duration `koanf:"dispatcher_heartbeat_interval" env:"RUNTIME_DISPATCHER_HEARTBEAT_INTERVAL" json:"dispatcher_heartbeat_interval" yaml:"dispatcher_heartbeat_interval" mapstructure:"dispatcher_heartbeat_interval"`

	// DispatcherHeartbeatTTL sets heartbeat expiration time.
	//
	// Must be greater than heartbeat interval to handle network delays.
	// Default: 90s
	DispatcherHeartbeatTTL time.Duration `koanf:"dispatcher_heartbeat_ttl" env:"RUNTIME_DISPATCHER_HEARTBEAT_TTL" json:"dispatcher_heartbeat_ttl" yaml:"dispatcher_heartbeat_ttl" mapstructure:"dispatcher_heartbeat_ttl"`

	// DispatcherStaleThreshold defines when a dispatcher is considered failed.
	//
	// Triggers reassignment of dispatcher's workflows.
	// Default: 120s
	DispatcherStaleThreshold time.Duration `koanf:"dispatcher_stale_threshold" env:"RUNTIME_DISPATCHER_STALE_THRESHOLD" json:"dispatcher_stale_threshold" yaml:"dispatcher_stale_threshold" mapstructure:"dispatcher_stale_threshold"`

	// AsyncTokenCounterWorkers sets the number of token counting workers.
	//
	// More workers improve throughput for high-volume token counting.
	// Default: 4
	AsyncTokenCounterWorkers int `koanf:"async_token_counter_workers" validate:"min=1" env:"RUNTIME_ASYNC_TOKEN_COUNTER_WORKERS" json:"async_token_counter_workers" yaml:"async_token_counter_workers" mapstructure:"async_token_counter_workers"`

	// AsyncTokenCounterBufferSize sets the token counter queue size.
	//
	// Larger buffers handle traffic spikes better but use more memory.
	// Default: 100
	AsyncTokenCounterBufferSize int `koanf:"async_token_counter_buffer_size" validate:"min=1" env:"RUNTIME_ASYNC_TOKEN_COUNTER_BUFFER_SIZE" json:"async_token_counter_buffer_size" yaml:"async_token_counter_buffer_size" mapstructure:"async_token_counter_buffer_size"`

	// ToolExecutionTimeout sets the maximum time for tool execution.
	//
	// Prevents runaway tools from blocking workflows.
	// Default: 60s
	ToolExecutionTimeout time.Duration `koanf:"tool_execution_timeout" env:"TOOL_EXECUTION_TIMEOUT" json:"tool_execution_timeout" yaml:"tool_execution_timeout" mapstructure:"tool_execution_timeout"`

	// RuntimeType specifies the JavaScript runtime to use for tool execution.
	//
	// Values: "bun", "node"
	// Default: "bun"
	RuntimeType string `koanf:"runtime_type" env:"RUNTIME_TYPE" json:"runtime_type" yaml:"runtime_type" mapstructure:"runtime_type" validate:"oneof=bun node"`

	// EntrypointPath specifies the path to the JavaScript/TypeScript entrypoint file.
	//
	// Default: "./tools.ts"
	EntrypointPath string `koanf:"entrypoint_path" env:"RUNTIME_ENTRYPOINT_PATH" json:"entrypoint_path" yaml:"entrypoint_path" mapstructure:"entrypoint_path"`

	// BunPermissions defines runtime security permissions for Bun.
	//
	// Default: ["--allow-read"]
	BunPermissions []string `koanf:"bun_permissions" env:"RUNTIME_BUN_PERMISSIONS" json:"bun_permissions" yaml:"bun_permissions" mapstructure:"bun_permissions"`
}

// LimitsConfig contains system limits and constraints.
//
// **System Resource Limits** prevent resource exhaustion and ensure stable operation
// under load. These limits protect against malicious inputs, programming errors,
// and resource-intensive workflows.
//
// ## Example Configuration
//
//	limits:
//	  max_string_length: 10485760      # 10MB
//	  max_message_content: 52428800    # 50MB
//	  max_total_content_size: 104857600 # 100MB
//	  max_nesting_depth: 10
//	  max_task_context_depth: 5
type LimitsConfig struct {
	// MaxNestingDepth limits JSON/YAML structure nesting.
	//
	// Prevents stack overflow from deeply nested data.
	// Default: 20
	MaxNestingDepth int `koanf:"max_nesting_depth" validate:"min=1" env:"LIMITS_MAX_NESTING_DEPTH" json:"max_nesting_depth" yaml:"max_nesting_depth" mapstructure:"max_nesting_depth"`

	// MaxStringLength limits individual string values.
	//
	// Applies to all string fields in requests and responses.
	// Default: 10MB (10485760 bytes)
	MaxStringLength int `koanf:"max_string_length" validate:"min=1" env:"LIMITS_MAX_STRING_LENGTH" json:"max_string_length" yaml:"max_string_length" mapstructure:"max_string_length"`

	// MaxMessageContent limits LLM message content size.
	//
	// Prevents excessive API costs and timeouts.
	// Default: 50MB (52428800 bytes)
	MaxMessageContent int `koanf:"max_message_content" validate:"min=1" env:"LIMITS_MAX_MESSAGE_CONTENT_LENGTH" json:"max_message_content" yaml:"max_message_content" mapstructure:"max_message_content"`

	// MaxTotalContentSize limits total request/response size.
	//
	// Prevents memory exhaustion from large payloads.
	// Default: 100MB (104857600 bytes)
	MaxTotalContentSize int `koanf:"max_total_content_size" validate:"min=1" env:"LIMITS_MAX_TOTAL_CONTENT_SIZE" json:"max_total_content_size" yaml:"max_total_content_size" mapstructure:"max_total_content_size"`

	// MaxTaskContextDepth limits task execution stack depth.
	//
	// Prevents infinite recursion in workflow execution.
	// Default: 10
	MaxTaskContextDepth int `koanf:"max_task_context_depth" validate:"min=1" env:"LIMITS_MAX_TASK_CONTEXT_DEPTH" json:"max_task_context_depth" yaml:"max_task_context_depth" mapstructure:"max_task_context_depth"`

	// ParentUpdateBatchSize controls database update batching.
	//
	// Larger batches improve throughput but increase memory usage.
	// Default: 100
	ParentUpdateBatchSize int `koanf:"parent_update_batch_size" validate:"min=1" env:"LIMITS_PARENT_UPDATE_BATCH_SIZE" json:"parent_update_batch_size" yaml:"parent_update_batch_size" mapstructure:"parent_update_batch_size"`
}

// MemoryConfig contains memory service configuration.
//
// **Agent Memory Service** provides conversational context and state management
// for AI agents across workflow executions. Memory enables agents to maintain
// context, remember previous interactions, and provide coherent responses.
//
// ## Example Configuration
//
//	memory:
//	  prefix: compozy:memory:
//	  ttl: 24h
//	  max_entries: 1000
type MemoryConfig struct {
	// Prefix namespaces memory keys in Redis.
	//
	// Prevents key collisions when sharing Redis.
	// Default: "compozy:memory:"
	Prefix string `koanf:"prefix" env:"MEMORY_PREFIX" json:"prefix" yaml:"prefix" mapstructure:"prefix"`

	// TTL sets memory entry expiration time.
	//
	// Balances context retention with storage costs.
	// Default: 24h
	TTL time.Duration `koanf:"ttl" env:"MEMORY_TTL" json:"ttl" yaml:"ttl" mapstructure:"ttl"`

	// MaxEntries limits memory entries per conversation.
	//
	// Prevents unbounded memory growth.
	// Default: 10000
	MaxEntries int `koanf:"max_entries" env:"MEMORY_MAX_ENTRIES" validate:"min=1" json:"max_entries" yaml:"max_entries" mapstructure:"max_entries"`
}

// LLMConfig contains LLM service configuration.
//
// **LLM Proxy Service** manages connections to language model providers
// and MCP (Model Context Protocol) servers. This enables unified access
// to multiple LLM providers and tool servers.
//
// ## Example Configuration
//
//	llm:
//	  proxy_url: http://localhost:6001
type LLMConfig struct {
	// ProxyURL specifies the MCP proxy server endpoint.
	//
	// The proxy handles:
	//   - MCP server connections
	//   - Tool discovery and routing
	//   - Protocol translation
	// Default: "" (empty; supply MCP proxy URL explicitly or derive from MCPProxy.BaseURL)
	ProxyURL string `koanf:"proxy_url" env:"MCP_PROXY_URL" json:"proxy_url" yaml:"proxy_url" mapstructure:"proxy_url"`

	// MCPReadinessTimeout bounds how long to wait for MCP clients to connect during setup.
	// Default: 60s
	MCPReadinessTimeout time.Duration `koanf:"mcp_readiness_timeout" env:"MCP_READINESS_TIMEOUT" json:"mcp_readiness_timeout" yaml:"mcp_readiness_timeout" mapstructure:"mcp_readiness_timeout"`

	// MCPReadinessPollInterval sets how often to poll the proxy for MCP connection status.
	// Default: 200ms
	MCPReadinessPollInterval time.Duration `koanf:"mcp_readiness_poll_interval" env:"MCP_READINESS_POLL_INTERVAL" json:"mcp_readiness_poll_interval" yaml:"mcp_readiness_poll_interval" mapstructure:"mcp_readiness_poll_interval"`

	// MCPHeaderTemplateStrict enables strict template validation for MCP HTTP headers.
	// When true, allows only simple lookups (no pipelines/function calls/inclusions).
	// Default: false
	MCPHeaderTemplateStrict bool `koanf:"mcp_header_template_strict" env:"MCP_HEADER_TEMPLATE_STRICT" json:"mcp_header_template_strict" yaml:"mcp_header_template_strict" mapstructure:"mcp_header_template_strict"`

	// RetryAttempts configures the number of retry attempts for LLM operations.
	//
	// Controls how many times the orchestrator will retry failed LLM requests
	// before giving up. Higher values improve reliability but may increase latency.
	// Default: 3
	RetryAttempts int `koanf:"retry_attempts" env:"LLM_RETRY_ATTEMPTS" json:"retry_attempts" yaml:"retry_attempts" mapstructure:"retry_attempts" validate:"min=0"`

	// RetryBackoffBase sets the base delay for exponential backoff retry strategy.
	//
	// The actual delay will be calculated as base * (2 ^ attempt) with optional jitter.
	// Lower values retry faster, higher values reduce server load.
	// Default: 100ms
	RetryBackoffBase time.Duration `koanf:"retry_backoff_base" env:"LLM_RETRY_BACKOFF_BASE" json:"retry_backoff_base" yaml:"retry_backoff_base" mapstructure:"retry_backoff_base"`

	// RetryBackoffMax limits the maximum delay between retry attempts.
	//
	// Prevents exponential backoff from creating extremely long delays.
	// Should be set based on user tolerance for response time.
	// Default: 10s
	RetryBackoffMax time.Duration `koanf:"retry_backoff_max" env:"LLM_RETRY_BACKOFF_MAX" json:"retry_backoff_max" yaml:"retry_backoff_max" mapstructure:"retry_backoff_max"`

	// RetryJitter enables random jitter in retry delays to avoid thundering herd.
	//
	// When enabled, adds randomness to retry delays to prevent all clients
	// from retrying simultaneously. Improves system stability under load.
	// Default: true
	RetryJitter bool `koanf:"retry_jitter" env:"LLM_RETRY_JITTER" json:"retry_jitter" yaml:"retry_jitter" mapstructure:"retry_jitter"`

	// MaxConcurrentTools limits the number of tools that can execute in parallel.
	//
	// Controls resource usage and prevents overwhelming downstream services.
	// Higher values improve throughput, lower values reduce resource contention.
	// Default: 10
	MaxConcurrentTools int `koanf:"max_concurrent_tools" env:"LLM_MAX_CONCURRENT_TOOLS" json:"max_concurrent_tools" yaml:"max_concurrent_tools" mapstructure:"max_concurrent_tools" validate:"min=0"`

	// MaxToolIterations caps the maximum number of tool-iteration loops per request.
	//
	// This acts as a global default and can be overridden by model-specific configuration
	// in project files. Set to 0 to use the orchestrator's built-in default.
	// Default: 100 (registry default)
	MaxToolIterations int `koanf:"max_tool_iterations" env:"LLM_MAX_TOOL_ITERATIONS" json:"max_tool_iterations" yaml:"max_tool_iterations" mapstructure:"max_tool_iterations" validate:"min=0"`

	// MaxSequentialToolErrors caps how many sequential tool execution/content errors
	// are tolerated for the same tool before aborting the task. Set to 0 to use
	// the orchestrator's built-in default.
	// Default: 10 (registry default)
	MaxSequentialToolErrors int `koanf:"max_sequential_tool_errors" env:"LLM_MAX_SEQUENTIAL_TOOL_ERRORS" json:"max_sequential_tool_errors" yaml:"max_sequential_tool_errors" mapstructure:"max_sequential_tool_errors" validate:"min=0"`

	// AllowedMCPNames restricts which MCP servers/tools are considered eligible
	// for advertisement and invocation. When empty, all discovered MCP tools
	// are eligible.
	AllowedMCPNames []string `koanf:"allowed_mcp_names" env:"LLM_ALLOWED_MCP_NAMES" json:"allowed_mcp_names" yaml:"allowed_mcp_names" mapstructure:"allowed_mcp_names"`

	// FailOnMCPRegistrationError enforces fail-fast behavior when registering
	// MCP configurations sourced from agents/projects. When true, MCP
	// registration failures cause service initialization to error.
	FailOnMCPRegistrationError bool `koanf:"fail_on_mcp_registration_error" env:"LLM_FAIL_ON_MCP_REGISTRATION_ERROR" json:"fail_on_mcp_registration_error" yaml:"fail_on_mcp_registration_error" mapstructure:"fail_on_mcp_registration_error"`

	// RegisterMCPs contains additional MCP configurations to be registered
	// with the MCP proxy at runtime. Represented as a generic slice to avoid
	// import cycles with engine packages.
	RegisterMCPs []map[string]any `koanf:"register_mcps" json:"register_mcps" yaml:"register_mcps" mapstructure:"register_mcps"`

	// MCPClientTimeout sets the HTTP client timeout for MCP proxy communication.
	MCPClientTimeout time.Duration `koanf:"mcp_client_timeout" env:"MCP_CLIENT_TIMEOUT" json:"mcp_client_timeout" yaml:"mcp_client_timeout" mapstructure:"mcp_client_timeout"`

	// RetryJitterPercent controls jitter strength when retry_jitter is enabled.
	RetryJitterPercent int `koanf:"retry_jitter_percent" env:"LLM_RETRY_JITTER_PERCENT" json:"retry_jitter_percent" yaml:"retry_jitter_percent" mapstructure:"retry_jitter_percent"`
}

// RateLimitConfig contains rate limiting configuration.
//
// **API Rate Limiting** protects the system from abuse and ensures fair usage
// across clients. Supports both global and per-API-key limits with Redis-backed
// distributed rate limiting.
//
// ## Example Configuration
//
//	ratelimit:
//	  global_rate:
//	    limit: 1000
//	    period: 1m
//	  api_key_rate:
//	    limit: 100
//	    period: 1m
//	  prefix: ratelimit:
//	  max_retry: 3
type RateLimitConfig struct {
	// GlobalRate applies to all requests system-wide.
	//
	// Protects against total system overload.
	GlobalRate RateConfig `koanf:"global_rate" env:"RATELIMIT_GLOBAL" json:"global_rate" yaml:"global_rate" mapstructure:"global_rate"`

	// APIKeyRate applies per API key.
	//
	// Ensures fair usage across different clients.
	APIKeyRate RateConfig `koanf:"api_key_rate" env:"RATELIMIT_API_KEY" json:"api_key_rate" yaml:"api_key_rate" mapstructure:"api_key_rate"`

	// Prefix namespaces rate limit keys in Redis.
	//
	// Default: "ratelimit:"
	Prefix string `koanf:"prefix" env:"RATELIMIT_PREFIX" json:"prefix" yaml:"prefix" mapstructure:"prefix"`

	// MaxRetry sets retry attempts for rate-limited requests.
	//
	// Default: 3
	MaxRetry int `koanf:"max_retry" env:"RATELIMIT_MAX_RETRY" json:"max_retry" yaml:"max_retry" mapstructure:"max_retry"`
}

// RateConfig represents a single rate limit configuration.
//
// **Rate Limit Definition** specifies the maximum number of requests allowed
// within a given time period. Used for both global and per-API-key limits.
//
// ## Example Configuration
//
//	rate:
//	  limit: 100      # 100 requests
//	  period: 1m      # per minute
type RateConfig struct {
	// Limit specifies the maximum number of requests allowed.
	//
	// Must be greater than 0.
	// Example: 100 for 100 requests per period
	Limit int64 `koanf:"limit" env:"LIMIT" json:"limit" yaml:"limit" mapstructure:"limit" validate:"min=1"`

	// Period defines the time window for the rate limit.
	//
	// Common values:
	//   - `"1s"`: Per second
	//   - `"1m"`: Per minute
	//   - `"1h"`: Per hour
	//   - `"24h"`: Per day
	Period time.Duration `koanf:"period" env:"PERIOD" json:"period" yaml:"period" mapstructure:"period" validate:"required"`
}

// RedisConfig contains Redis connection configuration.
//
// **Redis Configuration** defines how Compozy connects to Redis for caching,
// session management, rate limiting, and pub/sub messaging. Redis provides
// high-performance data storage and distributed coordination.
//
// ## Connection Methods
//
//  1. **Connection URL** (preferred for production):
//     ```yaml
//     redis:
//     url: "redis://user:pass@host:6379/0?sslmode=require"
//     ```
//
//  2. **Individual Parameters** (development/testing):
//     ```yaml
//     redis:
//     host: localhost
//     port: "6379"    # String format (required as of v2.0.0)
//     password: secret
//     db: 0
//     ```
//
//     **Note**: Port is now a string field. Both quoted strings ("6379") and
//     numeric literals (6379) are supported due to weakly-typed input parsing.
type RedisConfig struct {
	// URL provides a complete Redis connection string.
	//
	// Format: `redis://[user:password@]host:port/db`
	// Takes precedence over individual connection parameters.
	URL string `koanf:"url" json:"url" yaml:"url" mapstructure:"url" env:"REDIS_URL"`

	// Host specifies the Redis server hostname or IP address.
	//
	// Default: "localhost"
	Host string `koanf:"host" json:"host" yaml:"host" mapstructure:"host" env:"REDIS_HOST"`

	// Port specifies the Redis server port as a string.
	//
	// **Format**: String representation of port number (1-65535)
	// **YAML**: Both "6379" (quoted) and 6379 (numeric) are accepted
	// **Breaking Change**: Changed from int to string in v2.0.0
	//
	// Default: "6379"
	Port string `koanf:"port" json:"port" yaml:"port" mapstructure:"port" env:"REDIS_PORT"`

	// Password authenticates with Redis.
	//
	// **Security**: Use environment variables in production.
	Password string `koanf:"password" json:"password" yaml:"password" mapstructure:"password" env:"REDIS_PASSWORD"`

	// DB selects the Redis database number.
	//
	// Default: 0
	DB int `koanf:"db" json:"db" yaml:"db" mapstructure:"db" env:"REDIS_DB"`

	// MaxRetries sets the maximum number of retries before giving up.
	//
	// Default: 3
	MaxRetries int `koanf:"max_retries" json:"max_retries" yaml:"max_retries" mapstructure:"max_retries" env:"REDIS_MAX_RETRIES"`

	// PoolSize sets the maximum number of socket connections.
	//
	// Default: 10 per CPU
	PoolSize int `koanf:"pool_size" json:"pool_size" yaml:"pool_size" mapstructure:"pool_size" env:"REDIS_POOL_SIZE"`

	// MinIdleConns sets the minimum number of idle connections.
	//
	// Default: 0
	MinIdleConns int `koanf:"min_idle_conns" json:"min_idle_conns" yaml:"min_idle_conns" mapstructure:"min_idle_conns" env:"REDIS_MIN_IDLE_CONNS"`

	// MaxIdleConns sets the maximum number of idle connections.
	//
	// Default: 0
	MaxIdleConns int `koanf:"max_idle_conns" json:"max_idle_conns" yaml:"max_idle_conns" mapstructure:"max_idle_conns" env:"REDIS_MAX_IDLE_CONNS"`

	// DialTimeout sets timeout for establishing new connections.
	//
	// Default: 5s
	DialTimeout time.Duration `koanf:"dial_timeout" json:"dial_timeout" yaml:"dial_timeout" mapstructure:"dial_timeout" env:"REDIS_DIAL_TIMEOUT"`

	// ReadTimeout sets timeout for socket reads.
	//
	// Default: 3s
	ReadTimeout time.Duration `koanf:"read_timeout" json:"read_timeout" yaml:"read_timeout" mapstructure:"read_timeout" env:"REDIS_READ_TIMEOUT"`

	// WriteTimeout sets timeout for socket writes.
	//
	// Default: ReadTimeout
	WriteTimeout time.Duration `koanf:"write_timeout" json:"write_timeout" yaml:"write_timeout" mapstructure:"write_timeout" env:"REDIS_WRITE_TIMEOUT"`

	// PoolTimeout sets timeout for getting connection from pool.
	//
	// Default: ReadTimeout + 1s
	PoolTimeout time.Duration `koanf:"pool_timeout" json:"pool_timeout" yaml:"pool_timeout" mapstructure:"pool_timeout" env:"REDIS_POOL_TIMEOUT"`

	// PingTimeout sets timeout for ping command.
	//
	// Default: 1s
	PingTimeout time.Duration `koanf:"ping_timeout" json:"ping_timeout" yaml:"ping_timeout" mapstructure:"ping_timeout" env:"REDIS_PING_TIMEOUT"`

	// MinRetryBackoff sets minimum backoff between retries.
	//
	// Default: 8ms
	MinRetryBackoff time.Duration `koanf:"min_retry_backoff" json:"min_retry_backoff" yaml:"min_retry_backoff" mapstructure:"min_retry_backoff" env:"REDIS_MIN_RETRY_BACKOFF"`

	// MaxRetryBackoff sets maximum backoff between retries.
	//
	// Default: 512ms
	MaxRetryBackoff time.Duration `koanf:"max_retry_backoff" json:"max_retry_backoff" yaml:"max_retry_backoff" mapstructure:"max_retry_backoff" env:"REDIS_MAX_RETRY_BACKOFF"`

	// NotificationBufferSize sets buffer size for pub/sub notifications.
	//
	// Default: 100
	NotificationBufferSize int `koanf:"notification_buffer_size" json:"notification_buffer_size" yaml:"notification_buffer_size" mapstructure:"notification_buffer_size" env:"REDIS_NOTIFICATION_BUFFER_SIZE"`

	// TLSEnabled enables TLS encryption.
	//
	// Default: false
	TLSEnabled bool `koanf:"tls_enabled" json:"tls_enabled" yaml:"tls_enabled" mapstructure:"tls_enabled" env:"REDIS_TLS_ENABLED"`

	// TLSConfig provides custom TLS configuration.
	//
	// When TLSEnabled is true, this can be used to provide custom TLS settings.
	// If nil, default TLS configuration will be used.
	TLSConfig *tls.Config `koanf:"-" json:"-" yaml:"-" mapstructure:"-"`
}

// CacheConfig contains cache-specific configuration settings.
//
// **Distributed Caching** accelerates workflow execution by storing frequently accessed data.
// This configuration controls cache behavior and performance characteristics,
// while Redis connection settings are managed separately in RedisConfig.
//
// The cache layer supports:
//   - LLM response caching to reduce API costs
//   - Tool result caching for expensive computations
//   - Workflow state caching for distributed coordination
//   - Session data persistence across requests
//
// ## Example Configuration
//
//	cache:
//	  enabled: true
//	  ttl: 24h
//	  prefix: "compozy:cache:"
//	  max_item_size: 1048576  # 1MB
//	  compression_enabled: true
//	  compression_threshold: 1024  # 1KB
type CacheConfig struct {
	// Enabled determines if caching is active.
	//
	// When disabled, all cache operations become no-ops.
	// Useful for debugging or when Redis is unavailable.
	//
	// **Default**: `true`
	Enabled bool `koanf:"enabled" json:"enabled" yaml:"enabled" mapstructure:"enabled" env:"CACHE_ENABLED"`

	// TTL sets the default time-to-live for cached items.
	//
	// Balances data freshness with cache efficiency.
	// Can be overridden per operation.
	//
	// **Default**: `24h`
	TTL time.Duration `koanf:"ttl" json:"ttl" yaml:"ttl" mapstructure:"ttl" env:"CACHE_TTL"`

	// Prefix namespaces cache keys in Redis.
	//
	// Prevents key collisions when sharing Redis with other applications.
	// Format: `"<app>:<environment>:cache:"`
	//
	// **Default**: `"compozy:cache:"`
	Prefix string `koanf:"prefix" json:"prefix" yaml:"prefix" mapstructure:"prefix" env:"CACHE_PREFIX"`

	// MaxItemSize limits the maximum size of a single cached item.
	//
	// Prevents large objects from consuming excessive memory.
	// Items larger than this are not cached.
	//
	// **Default**: `1048576` (1MB)
	MaxItemSize int64 `koanf:"max_item_size" json:"max_item_size" yaml:"max_item_size" mapstructure:"max_item_size" env:"CACHE_MAX_ITEM_SIZE"`

	// CompressionEnabled activates data compression for cached items.
	//
	// Reduces memory usage and network bandwidth at the cost of CPU.
	// Uses gzip compression for items above CompressionThreshold.
	//
	// **Default**: `true`
	CompressionEnabled bool `koanf:"compression_enabled" json:"compression_enabled" yaml:"compression_enabled" mapstructure:"compression_enabled" env:"CACHE_COMPRESSION_ENABLED"`

	// CompressionThreshold sets the minimum size for compression.
	//
	// Items smaller than this are stored uncompressed to avoid
	// CPU overhead for minimal space savings.
	//
	// **Default**: `1024` (1KB)
	CompressionThreshold int64 `koanf:"compression_threshold" json:"compression_threshold" yaml:"compression_threshold" mapstructure:"compression_threshold" env:"CACHE_COMPRESSION_THRESHOLD"`

	// EvictionPolicy controls how items are removed when cache is full.
	//
	// Options:
	//   - `"lru"`: Least Recently Used (default)
	//   - `"lfu"`: Least Frequently Used
	//   - `"ttl"`: Time-based expiration only
	//
	// **Default**: `"lru"`
	EvictionPolicy string `koanf:"eviction_policy" json:"eviction_policy" yaml:"eviction_policy" mapstructure:"eviction_policy" env:"CACHE_EVICTION_POLICY"`

	// StatsInterval controls how often cache statistics are logged.
	//
	// Set to 0 to disable statistics logging.
	// Useful for monitoring cache hit rates and performance.
	//
	// **Default**: `5m`
	StatsInterval time.Duration `koanf:"stats_interval" json:"stats_interval" yaml:"stats_interval" mapstructure:"stats_interval" env:"CACHE_STATS_INTERVAL"`
}

// WorkerConfig contains Temporal worker configuration.
//
// **Temporal Worker Configuration** controls the behavior and performance characteristics
// of Temporal workers that execute workflows and activities. These settings affect
// timeouts, retry behavior, and operational thresholds for worker health monitoring.
//
// ## Example Configuration
//
//	worker:
//	  config_store_ttl: 24h
//	  heartbeat_cleanup_timeout: 5s
type WorkerConfig struct {
	// ConfigStoreTTL sets how long worker configurations are cached.
	//
	// **Default**: `24h`
	ConfigStoreTTL time.Duration `koanf:"config_store_ttl" json:"config_store_ttl" yaml:"config_store_ttl" mapstructure:"config_store_ttl" env:"WORKER_CONFIG_STORE_TTL"`

	// HeartbeatCleanupTimeout sets timeout for heartbeat cleanup operations.
	//
	// **Default**: `5s`
	HeartbeatCleanupTimeout time.Duration `koanf:"heartbeat_cleanup_timeout" json:"heartbeat_cleanup_timeout" yaml:"heartbeat_cleanup_timeout" mapstructure:"heartbeat_cleanup_timeout" env:"WORKER_HEARTBEAT_CLEANUP_TIMEOUT"`

	// MCPShutdownTimeout sets timeout for MCP server shutdown.
	//
	// **Default**: `30s`
	MCPShutdownTimeout time.Duration `koanf:"mcp_shutdown_timeout" json:"mcp_shutdown_timeout" yaml:"mcp_shutdown_timeout" mapstructure:"mcp_shutdown_timeout" env:"WORKER_MCP_SHUTDOWN_TIMEOUT"`

	// DispatcherRetryDelay sets delay between dispatcher retry attempts.
	//
	// **Default**: `50ms`
	DispatcherRetryDelay time.Duration `koanf:"dispatcher_retry_delay" json:"dispatcher_retry_delay" yaml:"dispatcher_retry_delay" mapstructure:"dispatcher_retry_delay" env:"WORKER_DISPATCHER_RETRY_DELAY"`

	// DispatcherMaxRetries sets maximum dispatcher retry attempts.
	//
	// **Default**: `2`
	DispatcherMaxRetries int `koanf:"dispatcher_max_retries" json:"dispatcher_max_retries" yaml:"dispatcher_max_retries" mapstructure:"dispatcher_max_retries" env:"WORKER_DISPATCHER_MAX_RETRIES"`

	// MCPProxyHealthCheckTimeout sets timeout for MCP proxy health checks.
	//
	// **Default**: `10s`
	MCPProxyHealthCheckTimeout time.Duration `koanf:"mcp_proxy_health_check_timeout" json:"mcp_proxy_health_check_timeout" yaml:"mcp_proxy_health_check_timeout" mapstructure:"mcp_proxy_health_check_timeout" env:"WORKER_MCP_PROXY_HEALTH_CHECK_TIMEOUT"`
}

// MCPProxyConfig contains MCP proxy server configuration.
//
// **MCP Proxy Configuration** defines settings for the Model Context Protocol proxy server
// that manages connections between Compozy and external MCP servers.
//
// ## Example Configuration
//
//	mcp_proxy:
//	  host: 0.0.0.0
//	  port: 6001
//	  base_url: http://localhost:6001
type MCPProxyConfig struct {
	// Host specifies the network interface to bind the MCP proxy server to.
	//
	// **Default**: `"0.0.0.0"`
	Host string `koanf:"host" json:"host" yaml:"host" mapstructure:"host" env:"MCP_PROXY_HOST"`

	// Port specifies the TCP port for the MCP proxy server.
	//
	// **Default**: `6001`
	Port int `koanf:"port" json:"port" yaml:"port" mapstructure:"port" env:"MCP_PROXY_PORT"`

	// BaseURL specifies the base URL for MCP proxy API endpoints.
	//
	// **Default**: `""` (empty string)
	BaseURL string `koanf:"base_url" json:"base_url" yaml:"base_url" mapstructure:"base_url" env:"MCP_PROXY_BASE_URL"`

	// ShutdownTimeout sets timeout for graceful shutdown.
	//
	// **Default**: `30s`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout" json:"shutdown_timeout" yaml:"shutdown_timeout" mapstructure:"shutdown_timeout" env:"MCP_PROXY_SHUTDOWN_TIMEOUT"`
}

// CLIConfig contains CLI-specific configuration.
//
// **Command-Line Interface Configuration** controls the behavior of the Compozy CLI tool,
// including authentication, output formatting, and interaction modes.
//
// ## Example Configuration
//
//	cli:
//	  api_key: ${COMPOZY_API_KEY}      # Secure API authentication
//	  base_url: https://api.compozy.com # API endpoint
//	  timeout: 30s                      # Request timeout
//	  mode: normal                      # Execution mode
//	  default_format: tui               # Output format
//	  color_mode: auto                  # Terminal color handling
//	  page_size: 50                     # Results per page
//	  debug: false                      # Debug logging
//	  quiet: false                      # Suppress output
//	  interactive: true                 # Interactive prompts
type CLIConfig struct {
	// APIKey authenticates CLI requests to the Compozy API.
	//
	// **Security**: Always use environment variables for API keys.
	// Never commit API keys to version control.
	APIKey SensitiveString `koanf:"api_key" env:"COMPOZY_API_KEY" json:"APIKey" yaml:"api_key" mapstructure:"api_key" sensitive:"true"`

	// BaseURL specifies the Compozy API endpoint.
	//
	// Default: "https://api.compozy.com"
	// Use custom endpoints for self-hosted or development environments.
	BaseURL string `koanf:"base_url" env:"COMPOZY_BASE_URL" json:"BaseURL" yaml:"base_url" mapstructure:"base_url"`

	// ServerURL overrides the server URL for local development.
	//
	// This takes precedence over BaseURL when set.
	// Example: "http://localhost:8080" for local server
	ServerURL string `koanf:"server_url" env:"COMPOZY_SERVER_URL" json:"ServerURL" yaml:"server_url" mapstructure:"server_url"`

	// Timeout sets the maximum duration for API requests.
	//
	// Default: 30s
	// Increase for long-running operations like workflow execution.
	Timeout time.Duration `koanf:"timeout" env:"COMPOZY_TIMEOUT" json:"Timeout" yaml:"timeout" mapstructure:"timeout"`

	// Mode controls the CLI execution behavior.
	//
	// Available modes:
	//   - `"normal"`: Standard interactive mode (default)
	//   - `"batch"`: Non-interactive batch processing
	//   - `"script"`: Optimized for scripting (minimal output)
	Mode string `koanf:"mode" env:"COMPOZY_MODE" json:"Mode" yaml:"mode" mapstructure:"mode"`

	// DefaultFormat sets the default output format.
	//
	// Options:
	//   - `"json"`: JSON format for programmatic consumption
	//   - `"tui"`: Terminal UI with tables and formatting (default)
	//   - `"auto"`: Automatically detect based on terminal capabilities
	DefaultFormat string `koanf:"default_format" env:"COMPOZY_DEFAULT_FORMAT" json:"DefaultFormat" yaml:"default_format" mapstructure:"default_format" validate:"oneof=json tui auto"`

	// ColorMode controls terminal color output.
	//
	// Options:
	//   - `"auto"`: Detect terminal color support (default)
	//   - `"on"`: Force color output
	//   - `"off"`: Disable all color output
	ColorMode string `koanf:"color_mode" env:"COMPOZY_COLOR_MODE" json:"ColorMode" yaml:"color_mode" mapstructure:"color_mode" validate:"oneof=auto on off"`

	// PageSize sets the number of results per page in list operations.
	//
	// Default: 50
	// Range: 1-1000
	PageSize int `koanf:"page_size" env:"COMPOZY_PAGE_SIZE" json:"PageSize" yaml:"page_size" mapstructure:"page_size" validate:"min=1,max=1000"`

	// OutputFormatAlias allows custom output format aliases.
	//
	// Used internally for format customization.
	OutputFormatAlias string `koanf:"output_format_alias" env:"" json:"OutputFormatAlias" yaml:"output_format_alias" mapstructure:"output_format_alias"`

	// NoColor disables all color output regardless of terminal support.
	//
	// Overrides ColorMode when set to true.
	NoColor bool `koanf:"no_color" env:"" json:"NoColor" yaml:"no_color" mapstructure:"no_color"`

	// Debug enables verbose debug logging.
	//
	// Shows detailed API requests, responses, and internal operations.
	Debug bool `koanf:"debug" env:"COMPOZY_DEBUG" json:"Debug" yaml:"debug" mapstructure:"debug"`

	// Quiet suppresses all non-error output.
	//
	// Useful for scripting and automation.
	Quiet bool `koanf:"quiet" env:"COMPOZY_QUIET" json:"Quiet" yaml:"quiet" mapstructure:"quiet"`

	// Interactive enables interactive prompts and confirmations.
	//
	// Default: true
	// Set to false for non-interactive environments.
	Interactive bool `koanf:"interactive" env:"COMPOZY_INTERACTIVE" json:"Interactive" yaml:"interactive" mapstructure:"interactive"`

	// ConfigFile specifies a custom configuration file path.
	//
	// Default: "./compozy.yaml" or "~/.compozy/config.yaml"
	ConfigFile string `koanf:"config_file" env:"COMPOZY_CONFIG_FILE" json:"ConfigFile" yaml:"config_file" mapstructure:"config_file"`

	// CWD overrides the current working directory.
	//
	// All relative paths will be resolved from this directory.
	CWD string `koanf:"cwd" env:"COMPOZY_CWD" json:"CWD" yaml:"cwd" mapstructure:"cwd"`

	// EnvFile specifies a .env file to load environment variables from.
	//
	// Variables in this file are loaded before processing configuration.
	EnvFile string `koanf:"env_file" env:"COMPOZY_ENV_FILE" json:"EnvFile" yaml:"env_file" mapstructure:"env_file"`
}

// Service defines the configuration management service interface.
// It provides methods for loading, watching, and validating configuration.
type Service interface {
	// Load loads configuration from the specified sources with precedence order.
	Load(ctx context.Context, sources ...Source) (*Config, error)
	// Watch monitors configuration changes and invokes callback on updates.
	Watch(ctx context.Context, callback func(*Config)) error
	// Validate checks if the configuration meets all validation requirements.
	Validate(config *Config) error
	// GetSource returns the source type for a specific configuration key.
	// This tracks which source (env, CLI, YAML, default) provided each value,
	// enabling debugging and precedence verification.
	GetSource(key string) SourceType
}

// Source defines the interface for configuration sources.
type Source interface {
	// Load reads configuration from the source.
	Load() (map[string]any, error)
	// Watch monitors the source for changes.
	Watch(ctx context.Context, callback func()) error
	// Type returns the source type identifier.
	Type() SourceType
	// Close releases any resources held by the source.
	Close() error
}

// SourceType identifies the type of configuration source.
type SourceType string

const (
	SourceCLI     SourceType = "cli"
	SourceYAML    SourceType = "yaml"
	SourceEnv     SourceType = "env"
	SourceDefault SourceType = "default"
)

// Metadata contains metadata about configuration sources.
type Metadata struct {
	Sources  map[string]SourceType `json:"sources"`
	LoadedAt time.Time             `json:"loaded_at"`
}

// Default returns a Config with default values for development.
func Default() *Config {
	return defaultFromRegistry()
}

// defaultFromRegistry creates a Config using the centralized registry
func defaultFromRegistry() *Config {
	registry := definition.CreateRegistry()
	return &Config{
		Server:    buildServerConfig(registry),
		Database:  buildDatabaseConfig(registry),
		Temporal:  buildTemporalConfig(registry),
		Runtime:   buildRuntimeConfig(registry),
		Limits:    buildLimitsConfig(registry),
		Memory:    buildMemoryConfig(registry),
		LLM:       buildLLMConfig(registry),
		RateLimit: buildRateLimitConfig(registry),
		CLI:       buildCLIConfig(registry),
		Redis:     buildRedisConfig(registry),
		Cache:     buildCacheConfig(registry),
		Worker:    buildWorkerConfig(registry),
		MCPProxy:  buildMCPProxyConfig(registry),
	}
}

// Helper functions for type-safe registry access
func getString(registry *definition.Registry, path string) string {
	if val := registry.GetDefault(path); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(registry *definition.Registry, path string) int {
	if val := registry.GetDefault(path); val != nil {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return 0
}

func getBool(registry *definition.Registry, path string) bool {
	if val := registry.GetDefault(path); val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func getDuration(registry *definition.Registry, path string) time.Duration {
	if val := registry.GetDefault(path); val != nil {
		if d, ok := val.(time.Duration); ok {
			return d
		}
	}
	return 0
}

func getInt64(registry *definition.Registry, path string) int64 {
	if val := registry.GetDefault(path); val != nil {
		if i, ok := val.(int64); ok {
			return i
		}
	}
	return 0
}

func getStringSlice(registry *definition.Registry, path string) []string {
	if val := registry.GetDefault(path); val != nil {
		if slice, ok := val.([]string); ok {
			return slice
		}
		// Handle case where it might be stored as []interface{}
		if interfaceSlice, ok := val.([]any); ok {
			result := make([]string, len(interfaceSlice))
			for i, v := range interfaceSlice {
				if s, ok := v.(string); ok {
					result[i] = s
				}
			}
			return result
		}
	}
	return []string{}
}

// getMapSlice returns []map[string]any from registry defaults, tolerating []any input.
func getMapSlice(registry *definition.Registry, path string) []map[string]any {
	if val := registry.GetDefault(path); val != nil {
		if m, ok := val.([]map[string]any); ok {
			return m
		}
		if arr, ok := val.([]any); ok {
			out := make([]map[string]any, 0, len(arr))
			for _, v := range arr {
				if mv, ok := v.(map[string]any); ok {
					out = append(out, mv)
				}
			}
			return out
		}
	}
	return nil
}

func buildServerConfig(registry *definition.Registry) ServerConfig {
	return ServerConfig{
		Host:        getString(registry, "server.host"),
		Port:        getInt(registry, "server.port"),
		CORSEnabled: getBool(registry, "server.cors_enabled"),
		CORS: CORSConfig{
			AllowedOrigins:   getStringSlice(registry, "server.cors.allowed_origins"),
			AllowCredentials: getBool(registry, "server.cors.allow_credentials"),
			MaxAge:           getInt(registry, "server.cors.max_age"),
		},
		Timeout: getDuration(registry, "server.timeout"),
		Auth: AuthConfig{
			Enabled:            getBool(registry, "server.auth.enabled"),
			WorkflowExceptions: getStringSlice(registry, "server.auth.workflow_exceptions"),
			AdminKey:           SensitiveString(getString(registry, "server.auth.admin_key")),
		},
	}
}

func buildDatabaseConfig(registry *definition.Registry) DatabaseConfig {
	return DatabaseConfig{
		Host:        getString(registry, "database.host"),
		Port:        getString(registry, "database.port"),
		User:        getString(registry, "database.user"),
		Password:    getString(registry, "database.password"),
		DBName:      getString(registry, "database.name"),
		SSLMode:     getString(registry, "database.ssl_mode"),
		AutoMigrate: getBool(registry, "database.auto_migrate"),
	}
}

func buildTemporalConfig(registry *definition.Registry) TemporalConfig {
	return TemporalConfig{
		HostPort:  getString(registry, "temporal.host_port"),
		Namespace: getString(registry, "temporal.namespace"),
		TaskQueue: getString(registry, "temporal.task_queue"),
	}
}

func buildRuntimeConfig(registry *definition.Registry) RuntimeConfig {
	return RuntimeConfig{
		Environment:                 getString(registry, "runtime.environment"),
		LogLevel:                    getString(registry, "runtime.log_level"),
		DispatcherHeartbeatInterval: getDuration(registry, "runtime.dispatcher_heartbeat_interval"),
		DispatcherHeartbeatTTL:      getDuration(registry, "runtime.dispatcher_heartbeat_ttl"),
		DispatcherStaleThreshold:    getDuration(registry, "runtime.dispatcher_stale_threshold"),
		AsyncTokenCounterWorkers:    getInt(registry, "runtime.async_token_counter_workers"),
		AsyncTokenCounterBufferSize: getInt(registry, "runtime.async_token_counter_buffer_size"),
		ToolExecutionTimeout:        getDuration(registry, "runtime.tool_execution_timeout"),
		RuntimeType:                 getString(registry, "runtime.runtime_type"),
		EntrypointPath:              getString(registry, "runtime.entrypoint_path"),
		BunPermissions:              getStringSlice(registry, "runtime.bun_permissions"),
	}
}

func buildLimitsConfig(registry *definition.Registry) LimitsConfig {
	return LimitsConfig{
		MaxNestingDepth:       getInt(registry, "limits.max_nesting_depth"),
		MaxStringLength:       getInt(registry, "limits.max_string_length"),
		MaxMessageContent:     getInt(registry, "limits.max_message_content"),
		MaxTotalContentSize:   getInt(registry, "limits.max_total_content_size"),
		MaxTaskContextDepth:   getInt(registry, "limits.max_task_context_depth"),
		ParentUpdateBatchSize: getInt(registry, "limits.parent_update_batch_size"),
	}
}

func buildMemoryConfig(registry *definition.Registry) MemoryConfig {
	return MemoryConfig{
		Prefix:     getString(registry, "memory.prefix"),
		TTL:        getDuration(registry, "memory.ttl"),
		MaxEntries: getInt(registry, "memory.max_entries"),
	}
}

func buildLLMConfig(registry *definition.Registry) LLMConfig {
	return LLMConfig{
		ProxyURL:                   getString(registry, "llm.proxy_url"),
		MCPReadinessTimeout:        getDuration(registry, "llm.mcp_readiness_timeout"),
		MCPReadinessPollInterval:   getDuration(registry, "llm.mcp_readiness_poll_interval"),
		MCPHeaderTemplateStrict:    getBool(registry, "llm.mcp_header_template_strict"),
		RetryAttempts:              getInt(registry, "llm.retry_attempts"),
		RetryBackoffBase:           getDuration(registry, "llm.retry_backoff_base"),
		RetryBackoffMax:            getDuration(registry, "llm.retry_backoff_max"),
		RetryJitter:                getBool(registry, "llm.retry_jitter"),
		MaxConcurrentTools:         getInt(registry, "llm.max_concurrent_tools"),
		MaxToolIterations:          getInt(registry, "llm.max_tool_iterations"),
		MaxSequentialToolErrors:    getInt(registry, "llm.max_sequential_tool_errors"),
		AllowedMCPNames:            getStringSlice(registry, "llm.allowed_mcp_names"),
		FailOnMCPRegistrationError: getBool(registry, "llm.fail_on_mcp_registration_error"),
		RegisterMCPs:               getMapSlice(registry, "llm.register_mcps"),
		MCPClientTimeout:           getDuration(registry, "llm.mcp_client_timeout"),
		RetryJitterPercent:         getInt(registry, "llm.retry_jitter_percent"),
	}
}

func buildRateLimitConfig(registry *definition.Registry) RateLimitConfig {
	return RateLimitConfig{
		GlobalRate: RateConfig{
			Limit:  getInt64(registry, "ratelimit.global_rate.limit"),
			Period: getDuration(registry, "ratelimit.global_rate.period"),
		},
		APIKeyRate: RateConfig{
			Limit:  getInt64(registry, "ratelimit.api_key_rate.limit"),
			Period: getDuration(registry, "ratelimit.api_key_rate.period"),
		},
		Prefix:   getString(registry, "ratelimit.prefix"),
		MaxRetry: getInt(registry, "ratelimit.max_retry"),
	}
}

func buildCLIConfig(registry *definition.Registry) CLIConfig {
	return CLIConfig{
		APIKey:            SensitiveString(getString(registry, "cli.api_key")),
		BaseURL:           getString(registry, "cli.base_url"),
		Timeout:           getDuration(registry, "cli.timeout"),
		Mode:              getString(registry, "cli.mode"),
		DefaultFormat:     getString(registry, "cli.default_format"),
		ColorMode:         getString(registry, "cli.color_mode"),
		PageSize:          getInt(registry, "cli.page_size"),
		OutputFormatAlias: getString(registry, "cli.output_format_alias"),
		NoColor:           getBool(registry, "cli.no_color"),
		Debug:             getBool(registry, "cli.debug"),
		Quiet:             getBool(registry, "cli.quiet"),
		Interactive:       getBool(registry, "cli.interactive"),
		ConfigFile:        getString(registry, "cli.config_file"),
		CWD:               getString(registry, "cli.cwd"),
		EnvFile:           getString(registry, "cli.env_file"),
	}
}

func buildRedisConfig(registry *definition.Registry) RedisConfig {
	return RedisConfig{
		URL:                    getString(registry, "redis.url"),
		Host:                   getString(registry, "redis.host"),
		Port:                   getString(registry, "redis.port"),
		Password:               getString(registry, "redis.password"),
		DB:                     getInt(registry, "redis.db"),
		MaxRetries:             getInt(registry, "redis.max_retries"),
		PoolSize:               getInt(registry, "redis.pool_size"),
		MinIdleConns:           getInt(registry, "redis.min_idle_conns"),
		MaxIdleConns:           getInt(registry, "redis.max_idle_conns"),
		DialTimeout:            getDuration(registry, "redis.dial_timeout"),
		ReadTimeout:            getDuration(registry, "redis.read_timeout"),
		WriteTimeout:           getDuration(registry, "redis.write_timeout"),
		PoolTimeout:            getDuration(registry, "redis.pool_timeout"),
		PingTimeout:            getDuration(registry, "redis.ping_timeout"),
		MinRetryBackoff:        getDuration(registry, "redis.min_retry_backoff"),
		MaxRetryBackoff:        getDuration(registry, "redis.max_retry_backoff"),
		NotificationBufferSize: getInt(registry, "redis.notification_buffer_size"),
		TLSEnabled:             getBool(registry, "redis.tls_enabled"),
	}
}

func buildCacheConfig(registry *definition.Registry) CacheConfig {
	return CacheConfig{
		Enabled:              getBool(registry, "cache.enabled"),
		TTL:                  getDuration(registry, "cache.ttl"),
		Prefix:               getString(registry, "cache.prefix"),
		MaxItemSize:          getInt64(registry, "cache.max_item_size"),
		CompressionEnabled:   getBool(registry, "cache.compression_enabled"),
		CompressionThreshold: getInt64(registry, "cache.compression_threshold"),
		EvictionPolicy:       getString(registry, "cache.eviction_policy"),
		StatsInterval:        getDuration(registry, "cache.stats_interval"),
	}
}

func buildWorkerConfig(registry *definition.Registry) WorkerConfig {
	return WorkerConfig{
		ConfigStoreTTL:             getDuration(registry, "worker.config_store_ttl"),
		HeartbeatCleanupTimeout:    getDuration(registry, "worker.heartbeat_cleanup_timeout"),
		MCPShutdownTimeout:         getDuration(registry, "worker.mcp_shutdown_timeout"),
		DispatcherRetryDelay:       getDuration(registry, "worker.dispatcher_retry_delay"),
		DispatcherMaxRetries:       getInt(registry, "worker.dispatcher_max_retries"),
		MCPProxyHealthCheckTimeout: getDuration(registry, "worker.mcp_proxy_health_check_timeout"),
	}
}

func buildMCPProxyConfig(registry *definition.Registry) MCPProxyConfig {
	return MCPProxyConfig{
		Host:            getString(registry, "mcp_proxy.host"),
		Port:            getInt(registry, "mcp_proxy.port"),
		BaseURL:         getString(registry, "mcp_proxy.base_url"),
		ShutdownTimeout: getDuration(registry, "mcp_proxy.shutdown_timeout"),
	}
}
