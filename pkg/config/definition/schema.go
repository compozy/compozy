package definition

import (
	"reflect"
	"time"
)

// CreateRegistry creates and populates the configuration registry
// This is the SINGLE SOURCE OF TRUTH for all configuration defaults
func CreateRegistry() *Registry {
	registry := NewRegistry()

	registerServerFields(registry)
	registerDatabaseFields(registry)
	registerTemporalFields(registry)
	registerRuntimeFields(registry)
	registerLimitsFields(registry)
	registerOpenAIFields(registry)
	registerMemoryFields(registry)
	registerLLMFields(registry)
	registerRateLimitFields(registry)
	registerCLIFields(registry)

	return registry
}

func registerServerFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "server.host",
		Default: "0.0.0.0",
		CLIFlag: "host",
		EnvVar:  "SERVER_HOST",
		Type:    reflect.TypeOf(""),
		Help:    "Host to bind the server to",
	})

	registry.Register(&FieldDef{
		Path:    "server.port",
		Default: 8080, // Single source of truth - no more 3001 vs 8080 confusion
		CLIFlag: "port",
		EnvVar:  "SERVER_PORT",
		Type:    reflect.TypeOf(0),
		Help:    "Port to run the server on",
	})

	registry.Register(&FieldDef{
		Path:    "server.cors_enabled",
		Default: true,
		CLIFlag: "cors",
		EnvVar:  "SERVER_CORS_ENABLED",
		Type:    reflect.TypeOf(true),
		Help:    "Enable CORS",
	})

	// CORS configuration fields
	registry.Register(&FieldDef{
		Path:    "server.cors.allowed_origins",
		Default: []string{"http://localhost:3000", "http://localhost:3001"}, // Development defaults
		CLIFlag: "cors-allowed-origins",
		EnvVar:  "SERVER_CORS_ALLOWED_ORIGINS",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Allowed CORS origins (comma-separated)",
	})

	registry.Register(&FieldDef{
		Path:    "server.cors.allow_credentials",
		Default: true,
		CLIFlag: "cors-allow-credentials",
		EnvVar:  "SERVER_CORS_ALLOW_CREDENTIALS",
		Type:    reflect.TypeOf(true),
		Help:    "Allow credentials in CORS requests",
	})

	registry.Register(&FieldDef{
		Path:    "server.cors.max_age",
		Default: 86400, // 24 hours
		CLIFlag: "cors-max-age",
		EnvVar:  "SERVER_CORS_MAX_AGE",
		Type:    reflect.TypeOf(0),
		Help:    "CORS preflight max age in seconds",
	})

	registry.Register(&FieldDef{
		Path:    "server.timeout",
		Default: 30 * time.Second,
		CLIFlag: "",
		EnvVar:  "SERVER_TIMEOUT",
		Type:    reflect.TypeOf(time.Second),
		Help:    "Server timeout",
	})
}

func registerDatabaseFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "database.host",
		Default: "localhost",
		CLIFlag: "db-host",
		EnvVar:  "DB_HOST",
		Type:    reflect.TypeOf(""),
		Help:    "Database host",
	})

	registry.Register(&FieldDef{
		Path:    "database.port",
		Default: "5432",
		CLIFlag: "db-port",
		EnvVar:  "DB_PORT",
		Type:    reflect.TypeOf(""),
		Help:    "Database port",
	})

	registry.Register(&FieldDef{
		Path:    "database.user",
		Default: "postgres",
		CLIFlag: "db-user",
		EnvVar:  "DB_USER",
		Type:    reflect.TypeOf(""),
		Help:    "Database user",
	})

	registry.Register(&FieldDef{
		Path:    "database.password",
		Default: "",
		CLIFlag: "db-password",
		EnvVar:  "DB_PASSWORD",
		Type:    reflect.TypeOf(""),
		Help:    "Database password",
	})

	registry.Register(&FieldDef{
		Path:    "database.name",
		Default: "compozy",
		CLIFlag: "db-name",
		EnvVar:  "DB_NAME",
		Type:    reflect.TypeOf(""),
		Help:    "Database name",
	})

	registry.Register(&FieldDef{
		Path:    "database.ssl_mode",
		Default: "disable",
		CLIFlag: "db-ssl-mode",
		EnvVar:  "DB_SSL_MODE",
		Type:    reflect.TypeOf(""),
		Help:    "Database SSL mode",
	})

	registry.Register(&FieldDef{
		Path:    "database.conn_string",
		Default: "",
		CLIFlag: "db-conn-string",
		EnvVar:  "DB_CONN_STRING",
		Type:    reflect.TypeOf(""),
		Help:    "Database connection string",
	})
}

func registerTemporalFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "temporal.host_port",
		Default: "localhost:7233",
		CLIFlag: "temporal-host",
		EnvVar:  "TEMPORAL_HOST_PORT",
		Type:    reflect.TypeOf(""),
		Help:    "Temporal host:port",
	})

	registry.Register(&FieldDef{
		Path:    "temporal.namespace",
		Default: "default",
		CLIFlag: "temporal-namespace",
		EnvVar:  "TEMPORAL_NAMESPACE",
		Type:    reflect.TypeOf(""),
		Help:    "Temporal namespace",
	})

	registry.Register(&FieldDef{
		Path:    "temporal.task_queue",
		Default: "compozy-tasks",
		CLIFlag: "temporal-task-queue",
		EnvVar:  "TEMPORAL_TASK_QUEUE",
		Type:    reflect.TypeOf(""),
		Help:    "Temporal task queue name",
	})
}

func registerRuntimeFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "runtime.environment",
		Default: "development",
		CLIFlag: "",
		EnvVar:  "RUNTIME_ENVIRONMENT",
		Type:    reflect.TypeOf(""),
		Help:    "Runtime environment",
	})

	registry.Register(&FieldDef{
		Path:    "runtime.log_level",
		Default: "info",
		CLIFlag: "log-level",
		EnvVar:  "RUNTIME_LOG_LEVEL",
		Type:    reflect.TypeOf(""),
		Help:    "Log level (debug, info, warn, error)",
	})

	registry.Register(&FieldDef{
		Path:    "runtime.dispatcher_heartbeat_interval",
		Default: 30 * time.Second,
		CLIFlag: "dispatcher-heartbeat-interval",
		EnvVar:  "RUNTIME_DISPATCHER_HEARTBEAT_INTERVAL",
		Type:    reflect.TypeOf(time.Second),
		Help:    "Dispatcher heartbeat interval",
	})

	registry.Register(&FieldDef{
		Path:    "runtime.dispatcher_heartbeat_ttl",
		Default: 90 * time.Second,
		CLIFlag: "dispatcher-heartbeat-ttl",
		EnvVar:  "RUNTIME_DISPATCHER_HEARTBEAT_TTL",
		Type:    reflect.TypeOf(time.Second),
		Help:    "Dispatcher heartbeat TTL",
	})

	registry.Register(&FieldDef{
		Path:    "runtime.dispatcher_stale_threshold",
		Default: 120 * time.Second,
		CLIFlag: "dispatcher-stale-threshold",
		EnvVar:  "RUNTIME_DISPATCHER_STALE_THRESHOLD",
		Type:    reflect.TypeOf(time.Second),
		Help:    "Dispatcher stale threshold",
	})

	registry.Register(&FieldDef{
		Path:    "runtime.async_token_counter_workers",
		Default: 4,
		CLIFlag: "async-token-counter-workers",
		EnvVar:  "RUNTIME_ASYNC_TOKEN_COUNTER_WORKERS",
		Type:    reflect.TypeOf(0),
		Help:    "Number of async token counter workers",
	})

	registry.Register(&FieldDef{
		Path:    "runtime.async_token_counter_buffer_size",
		Default: 100,
		CLIFlag: "async-token-counter-buffer-size",
		EnvVar:  "RUNTIME_ASYNC_TOKEN_COUNTER_BUFFER_SIZE",
		Type:    reflect.TypeOf(0),
		Help:    "Async token counter buffer size",
	})
}

func registerLimitsFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "limits.max_nesting_depth",
		Default: 20,
		CLIFlag: "max-nesting-depth",
		EnvVar:  "LIMITS_MAX_NESTING_DEPTH",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum nesting depth",
	})

	registry.Register(&FieldDef{
		Path:    "limits.max_string_length",
		Default: 10485760, // 10MB
		CLIFlag: "max-string-length",
		EnvVar:  "LIMITS_MAX_STRING_LENGTH",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum string length",
	})

	registry.Register(&FieldDef{
		Path:    "limits.max_message_content",
		Default: 10240, // 10KB
		CLIFlag: "max-message-content-length",
		EnvVar:  "LIMITS_MAX_MESSAGE_CONTENT_LENGTH",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum message content length",
	})

	registry.Register(&FieldDef{
		Path:    "limits.max_total_content_size",
		Default: 102400, // 100KB
		CLIFlag: "max-total-content-size",
		EnvVar:  "LIMITS_MAX_TOTAL_CONTENT_SIZE",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum total content size",
	})

	registry.Register(&FieldDef{
		Path:    "limits.max_task_context_depth",
		Default: 5,
		CLIFlag: "",
		EnvVar:  "LIMITS_MAX_TASK_CONTEXT_DEPTH",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum task context depth",
	})

	registry.Register(&FieldDef{
		Path:    "limits.parent_update_batch_size",
		Default: 100,
		CLIFlag: "",
		EnvVar:  "LIMITS_PARENT_UPDATE_BATCH_SIZE",
		Type:    reflect.TypeOf(0),
		Help:    "Parent update batch size",
	})
}

func registerOpenAIFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "openai.api_key",
		Default: "",
		CLIFlag: "openai-api-key",
		EnvVar:  "OPENAI_API_KEY",
		Type:    reflect.TypeOf(""),
		Help:    "OpenAI API key",
	})

	registry.Register(&FieldDef{
		Path:    "openai.base_url",
		Default: "",
		CLIFlag: "",
		EnvVar:  "OPENAI_BASE_URL",
		Type:    reflect.TypeOf(""),
		Help:    "OpenAI base URL",
	})

	registry.Register(&FieldDef{
		Path:    "openai.org_id",
		Default: "",
		CLIFlag: "",
		EnvVar:  "OPENAI_ORG_ID",
		Type:    reflect.TypeOf(""),
		Help:    "OpenAI organization ID",
	})

	registry.Register(&FieldDef{
		Path:    "openai.default_model",
		Default: "",
		CLIFlag: "",
		EnvVar:  "OPENAI_DEFAULT_MODEL",
		Type:    reflect.TypeOf(""),
		Help:    "OpenAI default model",
	})
}

func registerMemoryFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "memory.redis_url",
		Default: "",
		CLIFlag: "",
		EnvVar:  "MEMORY_REDIS_URL",
		Type:    reflect.TypeOf(""),
		Help:    "Redis URL for memory storage",
	})

	registry.Register(&FieldDef{
		Path:    "memory.redis_prefix",
		Default: "compozy:",
		CLIFlag: "",
		EnvVar:  "MEMORY_REDIS_PREFIX",
		Type:    reflect.TypeOf(""),
		Help:    "Redis key prefix",
	})

	registry.Register(&FieldDef{
		Path:    "memory.ttl",
		Default: 24 * time.Hour,
		CLIFlag: "",
		EnvVar:  "MEMORY_TTL",
		Type:    reflect.TypeOf(time.Hour),
		Help:    "Memory TTL",
	})

	registry.Register(&FieldDef{
		Path:    "memory.max_entries",
		Default: 10000,
		CLIFlag: "",
		EnvVar:  "MEMORY_MAX_ENTRIES",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum memory entries",
	})
}

func registerLLMFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.proxy_url",
		Default: "",
		CLIFlag: "",
		EnvVar:  "MCP_PROXY_URL",
		Type:    reflect.TypeOf(""),
		Help:    "LLM proxy URL",
	})

	registry.Register(&FieldDef{
		Path:    "llm.admin_token",
		Default: "",
		CLIFlag: "",
		EnvVar:  "MCP_ADMIN_TOKEN",
		Type:    reflect.TypeOf(""),
		Help:    "LLM admin token",
	})
}

func registerRateLimitFields(registry *Registry) {
	// Global rate limit
	registry.Register(&FieldDef{
		Path:    "ratelimit.global_rate.limit",
		Default: int64(100),
		CLIFlag: "",
		EnvVar:  "RATELIMIT_GLOBAL_LIMIT",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Global rate limit (requests per period)",
	})

	registry.Register(&FieldDef{
		Path:    "ratelimit.global_rate.period",
		Default: 1 * time.Minute,
		CLIFlag: "",
		EnvVar:  "RATELIMIT_GLOBAL_PERIOD",
		Type:    reflect.TypeOf(time.Minute),
		Help:    "Global rate limit period",
	})

	// API key rate limit
	registry.Register(&FieldDef{
		Path:    "ratelimit.api_key_rate.limit",
		Default: int64(100),
		CLIFlag: "",
		EnvVar:  "RATELIMIT_API_KEY_LIMIT",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "API key rate limit (requests per period)",
	})

	registry.Register(&FieldDef{
		Path:    "ratelimit.api_key_rate.period",
		Default: 1 * time.Minute,
		CLIFlag: "",
		EnvVar:  "RATELIMIT_API_KEY_PERIOD",
		Type:    reflect.TypeOf(time.Minute),
		Help:    "API key rate limit period",
	})

	// Redis configuration for rate limiting
	registry.Register(&FieldDef{
		Path:    "ratelimit.redis_addr",
		Default: "",
		CLIFlag: "",
		EnvVar:  "RATELIMIT_REDIS_ADDR",
		Type:    reflect.TypeOf(""),
		Help:    "Redis address for rate limit storage (optional)",
	})

	registry.Register(&FieldDef{
		Path:    "ratelimit.redis_password",
		Default: "",
		CLIFlag: "",
		EnvVar:  "RATELIMIT_REDIS_PASSWORD",
		Type:    reflect.TypeOf(""),
		Help:    "Redis password for rate limit storage",
	})

	registry.Register(&FieldDef{
		Path:    "ratelimit.redis_db",
		Default: 0,
		CLIFlag: "",
		EnvVar:  "RATELIMIT_REDIS_DB",
		Type:    reflect.TypeOf(0),
		Help:    "Redis database for rate limit storage",
	})

	registry.Register(&FieldDef{
		Path:    "ratelimit.prefix",
		Default: "compozy:ratelimit:",
		CLIFlag: "",
		EnvVar:  "RATELIMIT_PREFIX",
		Type:    reflect.TypeOf(""),
		Help:    "Key prefix for rate limit storage",
	})

	registry.Register(&FieldDef{
		Path:    "ratelimit.max_retry",
		Default: 3,
		CLIFlag: "",
		EnvVar:  "RATELIMIT_MAX_RETRY",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum retries for rate limit operations",
	})
}

func registerCLIFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "cli.api_key",
		Default: "",
		CLIFlag: "",
		EnvVar:  "COMPOZY_API_KEY",
		Type:    reflect.TypeOf(""),
		Help:    "Compozy API key for authentication",
	})

	registry.Register(&FieldDef{
		Path:    "cli.base_url",
		Default: "http://localhost:8080",
		CLIFlag: "",
		EnvVar:  "COMPOZY_BASE_URL",
		Type:    reflect.TypeOf(""),
		Help:    "Base URL for Compozy API",
	})

	registry.Register(&FieldDef{
		Path:    "cli.timeout",
		Default: 30 * time.Second,
		CLIFlag: "",
		EnvVar:  "COMPOZY_TIMEOUT",
		Type:    reflect.TypeOf(time.Second),
		Help:    "Timeout for API requests",
	})

	registry.Register(&FieldDef{
		Path:    "cli.mode",
		Default: "auto",
		CLIFlag: "",
		EnvVar:  "COMPOZY_MODE",
		Type:    reflect.TypeOf(""),
		Help:    "CLI mode: auto, json, or tui",
	})
}
