package config

import (
	"context"
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

	// OpenAI configures OpenAI API integration.
	//
	// $ref: schema://application#openai
	OpenAI OpenAIConfig `koanf:"openai" json:"openai" yaml:"openai" mapstructure:"openai"`

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
	Enabled            bool     `koanf:"enabled"             json:"enabled"             yaml:"enabled"             mapstructure:"enabled"             env:"SERVER_AUTH_ENABLED"`
	WorkflowExceptions []string `koanf:"workflow_exceptions" json:"workflow_exceptions" yaml:"workflow_exceptions" mapstructure:"workflow_exceptions" env:"SERVER_AUTH_WORKFLOW_EXCEPTIONS" validate:"dive,workflow_id"`
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
	// Default: "compozy-queue"
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
	// Default: 5s
	DispatcherHeartbeatInterval time.Duration `koanf:"dispatcher_heartbeat_interval" env:"RUNTIME_DISPATCHER_HEARTBEAT_INTERVAL" json:"dispatcher_heartbeat_interval" yaml:"dispatcher_heartbeat_interval" mapstructure:"dispatcher_heartbeat_interval"`

	// DispatcherHeartbeatTTL sets heartbeat expiration time.
	//
	// Must be greater than heartbeat interval to handle network delays.
	// Default: 15s (3x interval)
	DispatcherHeartbeatTTL time.Duration `koanf:"dispatcher_heartbeat_ttl" env:"RUNTIME_DISPATCHER_HEARTBEAT_TTL" json:"dispatcher_heartbeat_ttl" yaml:"dispatcher_heartbeat_ttl" mapstructure:"dispatcher_heartbeat_ttl"`

	// DispatcherStaleThreshold defines when a dispatcher is considered failed.
	//
	// Triggers reassignment of dispatcher's workflows.
	// Default: 30s
	DispatcherStaleThreshold time.Duration `koanf:"dispatcher_stale_threshold" env:"RUNTIME_DISPATCHER_STALE_THRESHOLD" json:"dispatcher_stale_threshold" yaml:"dispatcher_stale_threshold" mapstructure:"dispatcher_stale_threshold"`

	// AsyncTokenCounterWorkers sets the number of token counting workers.
	//
	// More workers improve throughput for high-volume token counting.
	// Default: 5
	AsyncTokenCounterWorkers int `koanf:"async_token_counter_workers" validate:"min=1" env:"RUNTIME_ASYNC_TOKEN_COUNTER_WORKERS" json:"async_token_counter_workers" yaml:"async_token_counter_workers" mapstructure:"async_token_counter_workers"`

	// AsyncTokenCounterBufferSize sets the token counter queue size.
	//
	// Larger buffers handle traffic spikes better but use more memory.
	// Default: 1000
	AsyncTokenCounterBufferSize int `koanf:"async_token_counter_buffer_size" validate:"min=1" env:"RUNTIME_ASYNC_TOKEN_COUNTER_BUFFER_SIZE" json:"async_token_counter_buffer_size" yaml:"async_token_counter_buffer_size" mapstructure:"async_token_counter_buffer_size"`

	// ToolExecutionTimeout sets the maximum time for tool execution.
	//
	// Prevents runaway tools from blocking workflows.
	// Default: 30s
	ToolExecutionTimeout time.Duration `koanf:"tool_execution_timeout" env:"TOOL_EXECUTION_TIMEOUT" json:"tool_execution_timeout" yaml:"tool_execution_timeout" mapstructure:"tool_execution_timeout"`
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

// OpenAIConfig contains OpenAI API configuration.
//
// **OpenAI Integration** enables GPT models for agent conversations and task execution.
// Configure API credentials and model preferences for your OpenAI usage.
//
// ## Example Configuration
//
//	openai:
//	  api_key: "{{ .env.OPENAI_API_KEY }}"
//	  default_model: gpt-4
//	  base_url: https://api.openai.com/v1  # Optional: for proxies
//	  org_id: org-xxxxx                    # Optional: for org billing
type OpenAIConfig struct {
	// APIKey authenticates with the OpenAI API.
	//
	// **Security**: Always use environment variables, never hardcode.
	// Get from: https://platform.openai.com/api-keys
	APIKey SensitiveString `koanf:"api_key" env:"OPENAI_API_KEY" sensitive:"true" json:"api_key" yaml:"api_key" mapstructure:"api_key"`

	// BaseURL overrides the OpenAI API endpoint.
	//
	// Use cases:
	//   - API proxies for corporate networks
	//   - Azure OpenAI endpoints
	//   - Local API gateways
	// Default: "https://api.openai.com/v1"
	BaseURL string `koanf:"base_url" env:"OPENAI_BASE_URL" json:"base_url" yaml:"base_url" mapstructure:"base_url"`

	// OrgID specifies the OpenAI organization for billing.
	//
	// Required for:
	//   - Multiple organization accounts
	//   - Usage tracking per organization
	OrgID string `koanf:"org_id" env:"OPENAI_ORG_ID" json:"org_id" yaml:"org_id" mapstructure:"org_id"`

	// DefaultModel sets the default GPT model for agents.
	//
	// Common models:
	//   - `"gpt-4"`: Most capable, higher cost
	//   - `"gpt-4-turbo"`: Faster GPT-4 variant
	//   - `"gpt-3.5-turbo"`: Fast and cost-effective
	// Default: "gpt-4"
	DefaultModel string `koanf:"default_model" env:"OPENAI_DEFAULT_MODEL" json:"default_model" yaml:"default_model" mapstructure:"default_model"`
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
//	  redis_url: redis://localhost:6379/1
//	  redis_prefix: compozy:memory:
//	  ttl: 24h
//	  max_entries: 1000
type MemoryConfig struct {
	// RedisURL specifies the Redis connection for memory storage.
	//
	// Format: `redis://[user:password@]host:port/db`
	// Default: "redis://localhost:6379/0"
	RedisURL string `koanf:"redis_url" env:"MEMORY_REDIS_URL" json:"redis_url" yaml:"redis_url" mapstructure:"redis_url"`

	// RedisPrefix namespaces memory keys in Redis.
	//
	// Prevents key collisions when sharing Redis.
	// Default: "compozy:memory:"
	RedisPrefix string `koanf:"redis_prefix" env:"MEMORY_REDIS_PREFIX" json:"redis_prefix" yaml:"redis_prefix" mapstructure:"redis_prefix"`

	// TTL sets memory entry expiration time.
	//
	// Balances context retention with storage costs.
	// Default: 24h
	TTL time.Duration `koanf:"ttl" env:"MEMORY_TTL" json:"ttl" yaml:"ttl" mapstructure:"ttl"`

	// MaxEntries limits memory entries per conversation.
	//
	// Prevents unbounded memory growth.
	// Default: 100
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
//	  proxy_url: http://localhost:8081
//	  admin_token: "{{ .env.MCP_ADMIN_TOKEN }}"
type LLMConfig struct {
	// ProxyURL specifies the MCP proxy server endpoint.
	//
	// The proxy handles:
	//   - MCP server connections
	//   - Tool discovery and routing
	//   - Protocol translation
	// Default: "http://localhost:8081"
	ProxyURL string `koanf:"proxy_url" env:"MCP_PROXY_URL" json:"proxy_url" yaml:"proxy_url" mapstructure:"proxy_url"`

	// AdminToken authenticates administrative operations.
	//
	// Required for:
	//   - MCP server registration
	//   - Proxy configuration changes
	//   - Debug endpoints
	// **Security**: Use environment variables
	AdminToken SensitiveString `koanf:"admin_token" env:"MCP_ADMIN_TOKEN" sensitive:"true" json:"admin_token" yaml:"admin_token" mapstructure:"admin_token"`
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
//	  redis_addr: localhost:6379
//	  redis_db: 2
type RateLimitConfig struct {
	// GlobalRate applies to all requests system-wide.
	//
	// Protects against total system overload.
	GlobalRate RateConfig `koanf:"global_rate" env:"RATELIMIT_GLOBAL" json:"global_rate" yaml:"global_rate" mapstructure:"global_rate"`

	// APIKeyRate applies per API key.
	//
	// Ensures fair usage across different clients.
	APIKeyRate RateConfig `koanf:"api_key_rate" env:"RATELIMIT_API_KEY" json:"api_key_rate" yaml:"api_key_rate" mapstructure:"api_key_rate"`

	// RedisAddr specifies the Redis server for rate limit storage.
	//
	// Format: "host:port"
	// Default: "localhost:6379"
	RedisAddr string `koanf:"redis_addr" env:"RATELIMIT_REDIS_ADDR" json:"redis_addr" yaml:"redis_addr" mapstructure:"redis_addr"`

	// RedisPassword authenticates with Redis.
	//
	// **Security**: Use environment variables
	RedisPassword string `koanf:"redis_password" env:"RATELIMIT_REDIS_PASSWORD" json:"redis_password" yaml:"redis_password" mapstructure:"redis_password"`

	// RedisDB selects the Redis database number.
	//
	// Use different DBs for different environments.
	// Default: 0
	RedisDB int `koanf:"redis_db" env:"RATELIMIT_REDIS_DB" json:"redis_db" yaml:"redis_db" mapstructure:"redis_db"`

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

// CLIConfig contains CLI-specific configuration.
//
// **Command-Line Interface Configuration** controls the behavior of the Compozy CLI tool,
// including authentication, output formatting, and interaction modes.
//
// ## Example Configuration
//
//	cli:
//	  api_key: ${COMPOZY_API_KEY}      # Secure API authentication
//	  base_url: https://api.compozy.dev # API endpoint
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
	// Default: "https://api.compozy.dev"
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

// Load loads configuration using the default service.
// This is a convenience function for simple configuration loading.
func Load() (*Config, error) {
	service := NewService()
	return service.Load(context.Background())
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
		OpenAI:    buildOpenAIConfig(registry),
		Memory:    buildMemoryConfig(registry),
		LLM:       buildLLMConfig(registry),
		RateLimit: buildRateLimitConfig(registry),
		CLI:       buildCLIConfig(registry),
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
		},
	}
}

func buildDatabaseConfig(registry *definition.Registry) DatabaseConfig {
	return DatabaseConfig{
		Host:     getString(registry, "database.host"),
		Port:     getString(registry, "database.port"),
		User:     getString(registry, "database.user"),
		Password: getString(registry, "database.password"),
		DBName:   getString(registry, "database.name"),
		SSLMode:  getString(registry, "database.ssl_mode"),
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

func buildOpenAIConfig(registry *definition.Registry) OpenAIConfig {
	return OpenAIConfig{
		APIKey:       SensitiveString(getString(registry, "openai.api_key")),
		BaseURL:      getString(registry, "openai.base_url"),
		OrgID:        getString(registry, "openai.org_id"),
		DefaultModel: getString(registry, "openai.default_model"),
	}
}

func buildMemoryConfig(registry *definition.Registry) MemoryConfig {
	return MemoryConfig{
		RedisURL:    getString(registry, "memory.redis_url"),
		RedisPrefix: getString(registry, "memory.redis_prefix"),
		TTL:         getDuration(registry, "memory.ttl"),
		MaxEntries:  getInt(registry, "memory.max_entries"),
	}
}

func buildLLMConfig(registry *definition.Registry) LLMConfig {
	return LLMConfig{
		ProxyURL:   getString(registry, "llm.proxy_url"),
		AdminToken: SensitiveString(getString(registry, "llm.admin_token")),
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
		RedisAddr:     getString(registry, "ratelimit.redis_addr"),
		RedisPassword: getString(registry, "ratelimit.redis_password"),
		RedisDB:       getInt(registry, "ratelimit.redis_db"),
		Prefix:        getString(registry, "ratelimit.prefix"),
		MaxRetry:      getInt(registry, "ratelimit.max_retry"),
	}
}

func buildCLIConfig(registry *definition.Registry) CLIConfig {
	return CLIConfig{
		APIKey:            SensitiveString(getString(registry, "cli.api_key")),
		BaseURL:           getString(registry, "cli.base_url"),
		ServerURL:         getString(registry, "cli.server_url"),
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
