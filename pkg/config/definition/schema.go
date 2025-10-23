package definition

import (
	"reflect"
	"time"
)

// Standard type definitions for consistency
var (
	durationType = reflect.TypeOf(time.Duration(0))
	float64Type  = reflect.TypeOf(float64(0))
)

// CreateRegistry creates and populates the configuration registry
// This is the SINGLE SOURCE OF TRUTH for all configuration defaults
func CreateRegistry() *Registry {
	registry := NewRegistry()
	registerServerFields(registry)
	registerDatabaseFields(registry)
	registerTemporalFields(registry)
	registerRuntimeFields(registry)
	registerStreamFields(registry)
	registerTasksFields(registry)
	registerLimitsFields(registry)
	registerAttachmentsFields(registry)
	registerMemoryFields(registry)
	registerKnowledgeFields(registry)
	registerLLMFields(registry)
	registerRateLimitFields(registry)
	registerCLIFields(registry)
	registerRedisFields(registry)
	registerCacheFields(registry)
	registerWorkerFields(registry)
	registerMCPProxyFields(registry)
	registerWebhooksFields(registry)
	return registry
}

func registerFieldDefs(registry *Registry, defs ...FieldDef) {
	for i := range defs {
		def := defs[i]
		registry.Register(&def)
	}
}

func registerStreamFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "stream.agent.default_poll",
			Default: 500 * time.Millisecond,
			EnvVar:  "STREAM_AGENT_DEFAULT_POLL",
			Type:    durationType,
			Help:    "Default polling interval for agent streaming when poll_ms is omitted",
		},
		FieldDef{
			Path:    "stream.agent.min_poll",
			Default: 250 * time.Millisecond,
			EnvVar:  "STREAM_AGENT_MIN_POLL",
			Type:    durationType,
			Help:    "Minimum allowed polling interval for agent streaming",
		},
		FieldDef{
			Path:    "stream.agent.max_poll",
			Default: 2000 * time.Millisecond,
			EnvVar:  "STREAM_AGENT_MAX_POLL",
			Type:    durationType,
			Help:    "Maximum allowed polling interval for agent streaming",
		},
		FieldDef{
			Path:    "stream.agent.heartbeat_frequency",
			Default: 15 * time.Second,
			EnvVar:  "STREAM_AGENT_HEARTBEAT_FREQUENCY",
			Type:    durationType,
			Help:    "Frequency for emitting heartbeat frames on agent streams",
		},
	)
}

func registerTasksFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "tasks.retry.child_state.max_attempts",
			Default: 5,
			EnvVar:  "TASKS_RETRY_CHILD_MAX_ATTEMPTS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum attempts to fetch child task state before failing",
		},
		FieldDef{
			Path:    "tasks.retry.child_state.base_backoff",
			Default: 50 * time.Millisecond,
			EnvVar:  "TASKS_RETRY_CHILD_BASE_BACKOFF",
			Type:    durationType,
			Help:    "Base exponential backoff used between child state fetch retries",
		},
		FieldDef{
			Path:    "tasks.wait.siblings.poll_interval",
			Default: 200 * time.Millisecond,
			EnvVar:  "TASKS_WAIT_SIBLINGS_POLL_INTERVAL",
			Type:    durationType,
			Help:    "Polling interval when waiting for prior sibling tasks to complete",
		},
		FieldDef{
			Path:    "tasks.wait.siblings.timeout",
			Default: 30 * time.Second,
			EnvVar:  "TASKS_WAIT_SIBLINGS_TIMEOUT",
			Type:    durationType,
			Help:    "Maximum time to wait for prior sibling tasks to finish",
		},
		FieldDef{
			Path:    "tasks.stream.max_chunks",
			Default: 100,
			EnvVar:  "TASKS_STREAM_MAX_CHUNKS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum text chunks to publish per task output stream (0 disables)",
		},
	)
}

func registerKnowledgeFields(registry *Registry) {
	registerKnowledgeBatchingFields(registry)
	registerKnowledgeChunkingFields(registry)
	registerKnowledgeRetrievalFields(registry)
	registerKnowledgeFileHandlingFields(registry)
	registerKnowledgeVectorTimeoutField(registry)
}

// registerKnowledgeBatchingFields registers embedder batching defaults.
// It keeps registerKnowledgeFields concise while grouping batching behavior.
func registerKnowledgeBatchingFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "knowledge.embedder_batch_size",
			Default: 512,
			CLIFlag: "knowledge-embedder-batch-size",
			EnvVar:  "KNOWLEDGE_EMBEDDER_BATCH_SIZE",
			Type:    reflect.TypeOf(0),
			Help:    "Default batch size for embedder requests when not set explicitly",
		},
	)
}

// registerKnowledgeChunkingFields configures knowledge chunk sizing defaults.
// It covers both chunk size and overlap controls used during ingestion.
func registerKnowledgeChunkingFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "knowledge.chunk_size",
			Default: 800,
			CLIFlag: "knowledge-chunk-size",
			EnvVar:  "KNOWLEDGE_CHUNK_SIZE",
			Type:    reflect.TypeOf(0),
			Help:    "Default chunk size applied to knowledge base sources that omit chunking.size",
		},
		FieldDef{
			Path:    "knowledge.chunk_overlap",
			Default: 120,
			CLIFlag: "knowledge-chunk-overlap",
			EnvVar:  "KNOWLEDGE_CHUNK_OVERLAP",
			Type:    reflect.TypeOf(0),
			Help:    "Default chunk overlap applied when chunking.overlap is not provided",
		},
	)
}

// registerKnowledgeRetrievalFields defines retrieval ranking defaults.
// These tune the number of returned chunks and score thresholds.
func registerKnowledgeRetrievalFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "knowledge.retrieval_top_k",
			Default: 5,
			CLIFlag: "knowledge-retrieval-top-k",
			EnvVar:  "KNOWLEDGE_RETRIEVAL_TOP_K",
			Type:    reflect.TypeOf(0),
			Help:    "Default number of results to return during knowledge retrieval when unspecified",
		},
		FieldDef{
			Path:    "knowledge.retrieval_min_score",
			Default: 0.0,
			CLIFlag: "knowledge-retrieval-min-score",
			EnvVar:  "KNOWLEDGE_RETRIEVAL_MIN_SCORE",
			Type:    float64Type,
			Help:    "Default minimum similarity score threshold when retrieval.min_score is not defined",
		},
	)
}

// registerKnowledgeFileHandlingFields governs ingestion file constraints.
// It enforces limits on markdown payload sizes accepted by the system.
func registerKnowledgeFileHandlingFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "knowledge.max_markdown_file_size_bytes",
			Default: 4 * 1024 * 1024,
			CLIFlag: "knowledge-max-markdown-file-size-bytes",
			EnvVar:  "KNOWLEDGE_MAX_MARKDOWN_FILE_SIZE_BYTES",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum markdown file size (in bytes) accepted during knowledge ingestion",
		},
	)
}

// registerKnowledgeVectorTimeoutField sets network timeouts for vector stores.
// It isolates HTTP timeout control for clarity alongside other knowledge helpers.
func registerKnowledgeVectorTimeoutField(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "knowledge.vector_http_timeout",
			Default: 10 * time.Second,
			CLIFlag: "knowledge-vector-http-timeout",
			EnvVar:  "KNOWLEDGE_VECTOR_HTTP_TIMEOUT",
			Type:    durationType,
			Help:    "HTTP client timeout applied to knowledge vector stores that use HTTP backends",
		},
	)
}

func registerServerFields(registry *Registry) {
	registerServerCoreFields(registry)
	registerServerTimeoutFields(registry)
	registerServerAuthFields(registry)
}

func registerServerCoreFields(registry *Registry) {
	registerServerHostPortCors(registry)
	registerServerTimeoutField(registry)
	registry.Register(
		&FieldDef{
			Path:    "server.source_of_truth",
			Default: "repo",
			CLIFlag: "",
			EnvVar:  "SERVER_SOURCE_OF_TRUTH",
			Type:    reflect.TypeOf(""),
			Help:    "Source of truth for workflows: repo|builder",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.seed_from_repo_on_empty",
			Default: false,
			CLIFlag: "",
			EnvVar:  "SERVER_SEED_FROM_REPO_ON_EMPTY",
			Type:    reflect.TypeOf(true),
			Help:    "If true, seed store from repo YAML once when empty (builder mode)",
		},
	)
}

func registerServerHostPortCors(registry *Registry) {
	registerServerHostPort(registry)
	registerServerCorsFields(registry)
}

// registerServerHostPort establishes the HTTP bind host and port.
// It isolates endpoint basics from the more verbose CORS configuration.
func registerServerHostPort(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "server.host",
			Default: "0.0.0.0",
			CLIFlag: "host",
			EnvVar:  "SERVER_HOST",
			Type:    reflect.TypeOf(""),
			Help:    "Host to bind the server to",
		},
		FieldDef{
			Path:    "server.port",
			Default: 5001,
			CLIFlag: "port",
			EnvVar:  "SERVER_PORT",
			Type:    reflect.TypeOf(0),
			Help:    "Port to run the server on",
		},
	)
}

// registerServerCorsFields defines allowed CORS behavior and metadata.
// Keeping it separate keeps the parent helper short and focused.
func registerServerCorsFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "server.cors_enabled",
			Default: true,
			CLIFlag: "cors",
			EnvVar:  "SERVER_CORS_ENABLED",
			Type:    reflect.TypeOf(true),
			Help:    "Enable CORS",
		},
		FieldDef{
			Path:    "server.cors.allowed_origins",
			Default: []string{"http://localhost:3000", "http://localhost:5001"},
			CLIFlag: "cors-allowed-origins",
			EnvVar:  "SERVER_CORS_ALLOWED_ORIGINS",
			Type:    reflect.TypeOf([]string{}),
			Help:    "Allowed CORS origins (comma-separated)",
		},
		FieldDef{
			Path:    "server.cors.allow_credentials",
			Default: true,
			CLIFlag: "cors-allow-credentials",
			EnvVar:  "SERVER_CORS_ALLOW_CREDENTIALS",
			Type:    reflect.TypeOf(true),
			Help:    "Allow credentials in CORS requests",
		},
		FieldDef{
			Path:    "server.cors.max_age",
			Default: 86400,
			CLIFlag: "cors-max-age",
			EnvVar:  "SERVER_CORS_MAX_AGE",
			Type:    reflect.TypeOf(0),
			Help:    "CORS preflight max age in seconds",
		},
	)
}

func registerServerTimeoutField(registry *Registry) {
	registry.Register(
		&FieldDef{
			Path:    "server.timeout",
			Default: 30 * time.Second,
			CLIFlag: "",
			EnvVar:  "SERVER_TIMEOUT",
			Type:    durationType,
			Help:    "Server timeout",
		},
	)
}

func registerServerAuthFields(registry *Registry) {
	// NOTE: Separate MCP client timeout avoids leaking readiness settings into HTTP requests.
	registry.Register(&FieldDef{
		Path:    "server.auth.enabled",
		Default: false, // Default to disabled in development
		CLIFlag: "auth-enabled",
		EnvVar:  "SERVER_AUTH_ENABLED",
		Type:    reflect.TypeOf(true),
		Help:    "Enable or disable authentication for API endpoints",
	})
	registry.Register(&FieldDef{
		Path:    "server.auth.workflow_exceptions",
		Default: []string{},
		CLIFlag: "auth-workflow-exceptions",
		EnvVar:  "SERVER_AUTH_WORKFLOW_EXCEPTIONS",
		Type:    reflect.TypeOf([]string{}),
		Help:    "List of workflow IDs that are exempt from authentication (comma-separated)",
	})
	registry.Register(&FieldDef{
		Path:    "server.auth.api_key_last_used_max_concurrency",
		Default: 10,
		CLIFlag: "auth-api-key-last-used-max-concurrency",
		EnvVar:  "SERVER_AUTH_API_KEY_LAST_USED_MAX_CONCURRENCY",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum concurrent API key last-used updates (0 disables async updates)",
	})
	registry.Register(&FieldDef{
		Path:    "server.auth.api_key_last_used_timeout",
		Default: 2 * time.Second,
		CLIFlag: "auth-api-key-last-used-timeout",
		EnvVar:  "SERVER_AUTH_API_KEY_LAST_USED_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout applied to asynchronous API key last-used updates",
	})
}

func registerServerTimeoutFields(registry *Registry) {
	registerServerShutdownTimeouts(registry)
	registerServerHTTPTimeouts(registry)
	registerServerScheduleTimeouts(registry)
	registerServerMiscTimeouts(registry)
}

func registerServerShutdownTimeouts(registry *Registry) {
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.monitoring_init",
			Default: 500 * time.Millisecond,
			EnvVar:  "SERVER_MONITORING_INIT_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for monitoring service initialization",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.monitoring_shutdown",
			Default: 5 * time.Second,
			EnvVar:  "SERVER_MONITORING_SHUTDOWN_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for monitoring service shutdown",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.db_shutdown",
			Default: 30 * time.Second,
			EnvVar:  "SERVER_DB_SHUTDOWN_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for database shutdown",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.worker_shutdown",
			Default: 30 * time.Second,
			EnvVar:  "SERVER_WORKER_SHUTDOWN_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for worker shutdown",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.server_shutdown",
			Default: 5 * time.Second,
			EnvVar:  "SERVER_SHUTDOWN_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for HTTP server graceful shutdown",
		},
	)
}

func registerServerHTTPTimeouts(registry *Registry) {
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.http_read",
			Default: 15 * time.Second,
			EnvVar:  "SERVER_HTTP_READ_TIMEOUT",
			Type:    durationType,
			Help:    "HTTP server read timeout",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.http_write",
			Default: 15 * time.Second,
			EnvVar:  "SERVER_HTTP_WRITE_TIMEOUT",
			Type:    durationType,
			Help:    "HTTP server write timeout",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.http_idle",
			Default: 60 * time.Second,
			EnvVar:  "SERVER_HTTP_IDLE_TIMEOUT",
			Type:    durationType,
			Help:    "HTTP server idle timeout",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.http_read_header",
			Default: 10 * time.Second,
			EnvVar:  "SERVER_HTTP_READ_HEADER_TIMEOUT",
			Type:    durationType,
			Help:    "HTTP server read header timeout to guard against Slowloris attacks",
		},
	)
}

func registerServerScheduleTimeouts(registry *Registry) {
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.schedule_retry_max_duration",
			Default: 5 * time.Minute,
			EnvVar:  "SERVER_SCHEDULE_RETRY_MAX_DURATION",
			Type:    durationType,
			Help:    "Max total duration for schedule reconciliation retries",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.schedule_retry_base_delay",
			Default: 1 * time.Second,
			EnvVar:  "SERVER_SCHEDULE_RETRY_BASE_DELAY",
			Type:    durationType,
			Help:    "Base delay for schedule reconciliation backoff",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.schedule_retry_max_delay",
			Default: 30 * time.Second,
			EnvVar:  "SERVER_SCHEDULE_RETRY_MAX_DELAY",
			Type:    durationType,
			Help:    "Max backoff delay for schedule reconciliation",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.schedule_retry_max_attempts",
			Default: 0,
			EnvVar:  "SERVER_SCHEDULE_RETRY_MAX_ATTEMPTS",
			Type:    reflect.TypeOf(0),
			Help:    "Max attempts for schedule reconciliation retries (0 = use max duration)",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.schedule_retry_backoff_seconds",
			Default: 0,
			EnvVar:  "SERVER_SCHEDULE_RETRY_BACKOFF_SECONDS",
			Type:    reflect.TypeOf(0),
			Help:    "Base backoff in seconds for reconciliation retries (0 = use base_delay)",
		},
	)
}

func registerServerMiscTimeouts(registry *Registry) {
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.knowledge_ingest",
			Default: 5 * time.Minute,
			EnvVar:  "SERVER_KNOWLEDGE_INGEST_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for startup knowledge ingestion runs (0 disables timeouts)",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.temporal_reachability",
			Default: 1500 * time.Millisecond,
			EnvVar:  "SERVER_TEMPORAL_REACHABILITY_TIMEOUT",
			Type:    durationType,
			Help:    "Reachability check timeout for Temporal",
		},
	)
	registry.Register(
		&FieldDef{
			Path:    "server.timeouts.start_probe_delay",
			Default: 100 * time.Millisecond,
			EnvVar:  "SERVER_START_PROBE_DELAY",
			Type:    durationType,
			Help:    "Delay before considering HTTP server started",
		},
	)
}

func registerDatabaseFields(registry *Registry) {
	registerDatabaseConnectionFields(registry)
	registerDatabaseOperationalFields(registry)
}

func registerDatabaseConnectionFields(registry *Registry) {
	registerDatabaseEndpointFields(registry)
	registerDatabaseCredentialFields(registry)
	registerDatabaseConnectionOverrideFields(registry)
}

// registerDatabaseEndpointFields captures core connection coordinates.
// It separates host and port data to keep the parent helper short.
func registerDatabaseEndpointFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "database.host",
			Default: "localhost",
			CLIFlag: "db-host",
			EnvVar:  "DB_HOST",
			Type:    reflect.TypeOf(""),
			Help:    "Database host",
		},
		FieldDef{
			Path:    "database.port",
			Default: "5432",
			CLIFlag: "db-port",
			EnvVar:  "DB_PORT",
			Type:    reflect.TypeOf(""),
			Help:    "Database port",
		},
	)
}

// registerDatabaseCredentialFields tracks auth and database selection settings.
// Grouping these makes credentials easy to review in isolation.
func registerDatabaseCredentialFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "database.user",
			Default: "postgres",
			CLIFlag: "db-user",
			EnvVar:  "DB_USER",
			Type:    reflect.TypeOf(""),
			Help:    "Database user",
		},
		FieldDef{
			Path:    "database.password",
			Default: "",
			CLIFlag: "db-password",
			EnvVar:  "DB_PASSWORD",
			Type:    reflect.TypeOf(""),
			Help:    "Database password",
		},
		FieldDef{
			Path:    "database.name",
			Default: "compozy",
			CLIFlag: "db-name",
			EnvVar:  "DB_NAME",
			Type:    reflect.TypeOf(""),
			Help:    "Database name",
		},
	)
}

// registerDatabaseConnectionOverrideFields covers SSL and DSN overrides.
// This separates optional overrides from the baseline connection pieces.
func registerDatabaseConnectionOverrideFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "database.ssl_mode",
			Default: "disable",
			CLIFlag: "db-ssl-mode",
			EnvVar:  "DB_SSL_MODE",
			Type:    reflect.TypeOf(""),
			Help:    "Database SSL mode",
		},
		FieldDef{
			Path:    "database.conn_string",
			Default: "",
			CLIFlag: "db-conn-string",
			EnvVar:  "DB_CONN_STRING",
			Type:    reflect.TypeOf(""),
			Help:    "Database connection string",
		},
	)
}

func registerDatabaseOperationalFields(registry *Registry) {
	registerDatabaseMigrationFields(registry)
	registerDatabasePoolSizeFields(registry)
	registerDatabasePoolLifetimeFields(registry)
	registerDatabasePoolTimeoutFields(registry)
}

// registerDatabaseMigrationFields controls migration automation knobs.
// It keeps operational automation options together for easy auditing.
func registerDatabaseMigrationFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "database.auto_migrate",
			Default: true,
			CLIFlag: "db-auto-migrate",
			EnvVar:  "DB_AUTO_MIGRATE",
			Type:    reflect.TypeOf(true),
			Help:    "Automatically run database migrations on startup",
		},
		FieldDef{
			Path:    "database.migration_timeout",
			Default: 2 * time.Minute,
			CLIFlag: "db-migration-timeout",
			EnvVar:  "DB_MIGRATION_TIMEOUT",
			Type:    durationType,
			Help:    "Maximum duration allowed for startup database migrations (must be >= 45s)",
		},
	)
}

// registerDatabasePoolSizeFields defines pool capacity constraints.
// These settings tune the number of concurrent database connections.
func registerDatabasePoolSizeFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "database.max_open_conns",
			Default: 25,
			CLIFlag: "db-max-open-conns",
			EnvVar:  "DB_MAX_OPEN_CONNS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum number of open PostgreSQL connections",
		},
		FieldDef{
			Path:    "database.max_idle_conns",
			Default: 5,
			CLIFlag: "db-max-idle-conns",
			EnvVar:  "DB_MAX_IDLE_CONNS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum number of idle connections kept in the pool",
		},
	)
}

// registerDatabasePoolLifetimeFields manages connection lifetime tuning.
// It keeps duration-based pool settings grouped for clarity.
func registerDatabasePoolLifetimeFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "database.conn_max_lifetime",
			Default: 5 * time.Minute,
			CLIFlag: "db-conn-max-lifetime",
			EnvVar:  "DB_CONN_MAX_LIFETIME",
			Type:    durationType,
			Help:    "Maximum lifetime for a pooled connection",
		},
		FieldDef{
			Path:    "database.conn_max_idle_time",
			Default: 1 * time.Minute,
			CLIFlag: "db-conn-max-idle-time",
			EnvVar:  "DB_CONN_MAX_IDLE_TIME",
			Type:    durationType,
			Help:    "Maximum idle time before a connection is recycled",
		},
	)
}

// registerDatabasePoolTimeoutFields covers timeout and period knobs for connectivity checks.
func registerDatabasePoolTimeoutFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "database.ping_timeout",
			Default: 3 * time.Second,
			CLIFlag: "db-ping-timeout",
			EnvVar:  "DB_PING_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for PostgreSQL ping during startup verification",
		},
		FieldDef{
			Path:    "database.health_check_timeout",
			Default: 1 * time.Second,
			CLIFlag: "db-health-check-timeout",
			EnvVar:  "DB_HEALTH_CHECK_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for runtime PostgreSQL health checks",
		},
		FieldDef{
			Path:    "database.health_check_period",
			Default: 30 * time.Second,
			CLIFlag: "db-health-check-period",
			EnvVar:  "DB_HEALTH_CHECK_PERIOD",
			Type:    durationType,
			Help:    "Interval between background pool health checks",
		},
		FieldDef{
			Path:    "database.connect_timeout",
			Default: 5 * time.Second,
			CLIFlag: "db-connect-timeout",
			EnvVar:  "DB_CONNECT_TIMEOUT",
			Type:    durationType,
			Help:    "Timeout for establishing new PostgreSQL connections",
		},
	)
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
	registerRuntimeCoreFields(registry)
	registerRuntimeDispatcherFields(registry)
	registerRuntimeToolFields(registry)
	registerRuntimeNativeToolsFields(registry)
}

func registerRuntimeCoreFields(registry *Registry) {
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
}

func registerRuntimeDispatcherFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "runtime.dispatcher_heartbeat_interval",
		Default: 30 * time.Second,
		CLIFlag: "dispatcher-heartbeat-interval",
		EnvVar:  "RUNTIME_DISPATCHER_HEARTBEAT_INTERVAL",
		Type:    durationType,
		Help:    "Dispatcher heartbeat interval",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.dispatcher_heartbeat_ttl",
		Default: 90 * time.Second,
		CLIFlag: "dispatcher-heartbeat-ttl",
		EnvVar:  "RUNTIME_DISPATCHER_HEARTBEAT_TTL",
		Type:    durationType,
		Help:    "Dispatcher heartbeat TTL",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.dispatcher_stale_threshold",
		Default: 120 * time.Second,
		CLIFlag: "dispatcher-stale-threshold",
		EnvVar:  "RUNTIME_DISPATCHER_STALE_THRESHOLD",
		Type:    durationType,
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

func registerRuntimeToolFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "runtime.tool_execution_timeout",
		Default: 60 * time.Second,
		CLIFlag: "tool-execution-timeout",
		EnvVar:  "TOOL_EXECUTION_TIMEOUT",
		Type:    durationType,
		Help:    "Tool execution timeout",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.task_execution_timeout_default",
		Default: 60 * time.Second,
		CLIFlag: "task-execution-timeout-default",
		EnvVar:  "TASK_EXECUTION_TIMEOUT_DEFAULT",
		Type:    durationType,
		Help:    "Default timeout for direct task executions",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.task_execution_timeout_max",
		Default: 300 * time.Second,
		CLIFlag: "task-execution-timeout-max",
		EnvVar:  "TASK_EXECUTION_TIMEOUT_MAX",
		Type:    durationType,
		Help:    "Maximum timeout allowed for direct task executions",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.runtime_type",
		Default: "bun",
		CLIFlag: "runtime-type",
		EnvVar:  "RUNTIME_TYPE",
		Type:    reflect.TypeOf(""),
		Help:    "JavaScript runtime to use for tool execution (bun, node)",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.entrypoint_path",
		Default: "",
		CLIFlag: "entrypoint-path",
		EnvVar:  "RUNTIME_ENTRYPOINT_PATH",
		Type:    reflect.TypeOf(""),
		Help:    "Path to the JavaScript/TypeScript entrypoint file",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.bun_permissions",
		Default: []string{"--allow-read"},
		CLIFlag: "bun-permissions",
		EnvVar:  "RUNTIME_BUN_PERMISSIONS",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Bun runtime security permissions",
	})
}

func registerRuntimeNativeToolsFields(registry *Registry) {
	registerRuntimeNativeToolsCoreFields(registry)
	registerRuntimeNativeToolsExecFields(registry)
	registerRuntimeNativeToolsFetchFields(registry)
	registerRuntimeNativeToolsCallAgentFields(registry)
}

func registerRuntimeNativeToolsCoreFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.enabled",
		Default: true,
		CLIFlag: "native-tools-enabled",
		EnvVar:  "RUNTIME_NATIVE_TOOLS_ENABLED",
		Type:    reflect.TypeOf(true),
		Help:    "Enable native cp__ builtin tools",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.root_dir",
		Default: ".",
		CLIFlag: "native-tools-root",
		EnvVar:  "RUNTIME_NATIVE_TOOLS_ROOT_DIR",
		Type:    reflect.TypeOf(""),
		Help:    "Root directory sandbox for native tools",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.additional_roots",
		Default: []string{},
		CLIFlag: "native-tools-additional-roots",
		EnvVar:  "RUNTIME_NATIVE_TOOLS_ADDITIONAL_ROOTS",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Additional sandbox roots that native tools may access",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.exec.timeout",
		Default: 30 * time.Second,
		EnvVar:  "RUNTIME_NATIVE_TOOLS_EXEC_TIMEOUT",
		Type:    reflect.TypeOf(time.Duration(0)),
		Help:    "Default timeout applied to cp__exec commands",
	})
}

func registerRuntimeNativeToolsExecFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.exec.max_stdout_bytes",
		Default: int64(2 << 20),
		EnvVar:  "RUNTIME_NATIVE_TOOLS_EXEC_MAX_STDOUT_BYTES",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Maximum stdout bytes captured for cp__exec",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.exec.max_stderr_bytes",
		Default: int64(1 << 10),
		EnvVar:  "RUNTIME_NATIVE_TOOLS_EXEC_MAX_STDERR_BYTES",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Maximum stderr bytes captured for cp__exec",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.exec.allowlist",
		Default: []map[string]any{},
		CLIFlag: "native-tools-exec-allowlist",
		EnvVar:  "RUNTIME_NATIVE_TOOLS_EXEC_ALLOWLIST",
		Type:    reflect.TypeOf([]map[string]any{}),
		Help:    "Additional command policies appended to the cp__exec allowlist",
	})
}

func registerRuntimeNativeToolsFetchFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.fetch.timeout",
		Default: 5 * time.Second,
		EnvVar:  "RUNTIME_NATIVE_TOOLS_FETCH_TIMEOUT",
		Type:    reflect.TypeOf(time.Duration(0)),
		Help:    "Default timeout applied to cp__fetch requests",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.fetch.max_body_bytes",
		Default: int64(2 << 20),
		EnvVar:  "RUNTIME_NATIVE_TOOLS_FETCH_MAX_BODY_BYTES",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Maximum response body bytes returned by cp__fetch",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.fetch.max_redirects",
		Default: 5,
		EnvVar:  "RUNTIME_NATIVE_TOOLS_FETCH_MAX_REDIRECTS",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum redirects cp__fetch will follow",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.fetch.allowed_methods",
		Default: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		EnvVar:  "RUNTIME_NATIVE_TOOLS_FETCH_ALLOWED_METHODS",
		Type:    reflect.TypeOf([]string{}),
		Help:    "HTTP methods permitted for cp__fetch",
	})
}

func registerRuntimeNativeToolsCallAgentFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.call_agent.enabled",
		Default: true,
		EnvVar:  "RUNTIME_NATIVE_TOOLS_CALL_AGENT_ENABLED",
		Type:    reflect.TypeOf(true),
		Help:    "Enable cp__call_agent builtin",
	})
	registry.Register(&FieldDef{
		Path:    "runtime.native_tools.call_agent.default_timeout",
		Default: 60 * time.Second,
		EnvVar:  "RUNTIME_NATIVE_TOOLS_CALL_AGENT_DEFAULT_TIMEOUT",
		Type:    durationType,
		Help:    "Default timeout applied to cp__call_agent executions when not specified",
	})
}

func registerLimitsFields(registry *Registry) {
	registerLimitsDepthFields(registry)
	registerLimitsPayloadFields(registry)
	registerLimitsBatchFields(registry)
}

// registerLimitsDepthFields covers recursion depth protections.
// It groups task and configuration depth caps for easy inspection.
func registerLimitsDepthFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "limits.max_nesting_depth",
			Default: 20,
			CLIFlag: "max-nesting-depth",
			EnvVar:  "LIMITS_MAX_NESTING_DEPTH",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum nesting depth",
		},
		FieldDef{
			Path:    "limits.max_config_file_nesting_depth",
			Default: 100,
			CLIFlag: "max-config-file-nesting-depth",
			EnvVar:  "LIMITS_MAX_CONFIG_FILE_NESTING_DEPTH",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum nesting depth when parsing configuration files",
		},
		FieldDef{
			Path:    "limits.max_task_context_depth",
			Default: 5,
			EnvVar:  "LIMITS_MAX_TASK_CONTEXT_DEPTH",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum task context depth",
		},
	)
}

// registerLimitsPayloadFields enforces per-payload size restrictions.
// These limits guard string, config, and message payload growth.
func registerLimitsPayloadFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "limits.max_string_length",
			Default: 10_485_760,
			CLIFlag: "max-string-length",
			EnvVar:  "LIMITS_MAX_STRING_LENGTH",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum string length",
		},
		FieldDef{
			Path:    "limits.max_config_file_size",
			Default: 10_485_760,
			CLIFlag: "max-config-file-size",
			EnvVar:  "LIMITS_MAX_CONFIG_FILE_SIZE",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum configuration file size in bytes",
		},
		FieldDef{
			Path:    "limits.max_message_content",
			Default: 10_240,
			CLIFlag: "max-message-content-length",
			EnvVar:  "LIMITS_MAX_MESSAGE_CONTENT_LENGTH",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum message content length",
		},
		FieldDef{
			Path:    "limits.max_total_content_size",
			Default: 102_400,
			CLIFlag: "max-total-content-size",
			EnvVar:  "LIMITS_MAX_TOTAL_CONTENT_SIZE",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum total content size",
		},
	)
}

// registerLimitsBatchFields tracks parent/update batching controls.
// Keeping it isolated highlights operational batching defaults.
func registerLimitsBatchFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "limits.parent_update_batch_size",
			Default: 100,
			EnvVar:  "LIMITS_PARENT_UPDATE_BATCH_SIZE",
			Type:    reflect.TypeOf(0),
			Help:    "Parent update batch size",
		},
	)
}

// registerAttachmentsFields registers global attachment-related configuration fields.
// Follows patterns from .cursor/rules/global-config.mdc
func registerAttachmentsFields(registry *Registry) {
	registerAttachmentsLimits(registry)
	registerAttachmentsMime(registry)
	registerAttachmentsQuota(registry)
	registerAttachmentsExtras(registry)
}

func registerAttachmentsLimits(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "attachments.max_download_size_bytes",
		Default: int64(10_000_000),
		CLIFlag: "attachments-max-download-size",
		EnvVar:  "ATTACHMENTS_MAX_DOWNLOAD_SIZE_BYTES",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Maximum download size in bytes for attachment resolution",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.download_timeout",
		Default: 30 * time.Second,
		CLIFlag: "attachments-download-timeout",
		EnvVar:  "ATTACHMENTS_DOWNLOAD_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for downloading attachments",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.max_redirects",
		Default: 3,
		CLIFlag: "attachments-max-redirects",
		EnvVar:  "ATTACHMENTS_MAX_REDIRECTS",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum number of HTTP redirects when downloading attachments",
	})
}

func registerAttachmentsMime(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "attachments.allowed_mime_types.image",
		Default: []string{"image/*"},
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_ALLOWED_MIME_TYPES_IMAGE",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Allowed MIME types for image attachments",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.allowed_mime_types.audio",
		Default: []string{"audio/*"},
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_ALLOWED_MIME_TYPES_AUDIO",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Allowed MIME types for audio attachments",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.allowed_mime_types.video",
		Default: []string{"video/*"},
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_ALLOWED_MIME_TYPES_VIDEO",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Allowed MIME types for video attachments",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.allowed_mime_types.pdf",
		Default: []string{"application/pdf"},
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_ALLOWED_MIME_TYPES_PDF",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Allowed MIME types for PDF attachments",
	})
}

func registerAttachmentsQuota(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "attachments.temp_dir_quota_bytes",
		Default: int64(0),
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_TEMP_DIR_QUOTA_BYTES",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Optional quota in bytes for temporary files used during attachment resolution (0 = unlimited)",
	})
}

func registerAttachmentsExtras(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "attachments.text_part_max_bytes",
		Default: int64(5 * 1024 * 1024),
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_TEXT_PART_MAX_BYTES",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Maximum text bytes loaded from files when converting to TextPart",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.pdf_extract_max_chars",
		Default: 1_000_000,
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_PDF_EXTRACT_MAX_CHARS",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum characters extracted from PDF when converting to text",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.http_user_agent",
		Default: "Compozy/1.0",
		CLIFlag: "attachments-http-user-agent",
		EnvVar:  "ATTACHMENTS_HTTP_USER_AGENT",
		Type:    reflect.TypeOf(""),
		Help:    "User-Agent header for attachment HTTP requests",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.mime_head_max_bytes",
		Default: 512,
		CLIFlag: "",
		EnvVar:  "ATTACHMENTS_MIME_HEAD_MAX_BYTES",
		Type:    reflect.TypeOf(0),
		Help:    "Number of initial bytes used for MIME detection",
	})
	registry.Register(&FieldDef{
		Path:    "attachments.ssrf_strict",
		Default: false,
		CLIFlag: "attachments-ssrf-strict",
		EnvVar:  "ATTACHMENTS_SSRF_STRICT",
		Type:    reflect.TypeOf(true),
		Help:    "Block local/loopback destinations even during tests (anti-SSRF)",
	})
}

func registerMemoryFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "memory.prefix",
		Default: "compozy:memory:",
		CLIFlag: "",
		EnvVar:  "MEMORY_PREFIX",
		Type:    reflect.TypeOf(""),
		Help:    "Redis key prefix for memory storage",
	})
	registry.Register(&FieldDef{
		Path:    "memory.ttl",
		Default: 24 * time.Hour,
		CLIFlag: "",
		EnvVar:  "MEMORY_TTL",
		Type:    durationType,
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
		CLIFlag: "llm-proxy-url",
		EnvVar:  "MCP_PROXY_URL",
		Type:    reflect.TypeOf(""),
		Help:    "LLM proxy URL",
	})
	registry.Register(&FieldDef{
		Path:    "llm.mcp_readiness_timeout",
		Default: 60 * time.Second,
		CLIFlag: "llm-mcp-readiness-timeout",
		EnvVar:  "MCP_READINESS_TIMEOUT",
		Type:    durationType,
		Help:    "Max time to wait for MCP clients to connect",
	})
	registry.Register(&FieldDef{
		Path:    "llm.mcp_readiness_poll_interval",
		Default: 200 * time.Millisecond,
		CLIFlag: "llm-mcp-readiness-poll-interval",
		EnvVar:  "MCP_READINESS_POLL_INTERVAL",
		Type:    durationType,
		Help:    "Polling interval for MCP connection readiness",
	})
	registry.Register(&FieldDef{
		Path:    "llm.mcp_header_template_strict",
		Default: false,
		CLIFlag: "llm-mcp-header-template-strict",
		EnvVar:  "MCP_HEADER_TEMPLATE_STRICT",
		Type:    reflect.TypeOf(true),
		Help:    "Enable strict template validation for MCP headers",
	})
	registerLLMRetryAndLimits(registry)
	registerLLMRateLimiterFields(registry)
	registerLLMConversationControls(registry)
	registerLLMMCPExtras(registry)
	registerLLMDefaultParams(registry)
}

// registerLLMMCPExtras splits MCP-related LLM fields to keep function sizes small
func registerLLMMCPExtras(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.allowed_mcp_names",
		Default: []string{},
		CLIFlag: "llm-allowed-mcp-names",
		EnvVar:  "LLM_ALLOWED_MCP_NAMES",
		Type:    reflect.TypeOf([]string{}),
		Help:    "Allowed MCP IDs for tool advertisement and lookup",
	})
	registry.Register(&FieldDef{
		Path:    "llm.fail_on_mcp_registration_error",
		Default: false,
		CLIFlag: "llm-fail-on-mcp-registration-error",
		EnvVar:  "LLM_FAIL_ON_MCP_REGISTRATION_ERROR",
		Type:    reflect.TypeOf(true),
		Help:    "Fail-fast when MCP registration encounters an error",
	})
	registry.Register(&FieldDef{
		Path:    "llm.register_mcps",
		Default: []any{},
		CLIFlag: "",
		EnvVar:  "",
		Type:    reflect.TypeOf([]any{}),
		Help:    "Additional MCP configurations to register with the proxy",
	})
	registry.Register(&FieldDef{
		Path:    "llm.mcp_client_timeout",
		Default: 30 * time.Second,
		CLIFlag: "llm-mcp-client-timeout",
		EnvVar:  "MCP_CLIENT_TIMEOUT",
		Type:    durationType,
		Help:    "HTTP client timeout for MCP proxy communication",
	})
	registry.Register(&FieldDef{
		Path:    "llm.retry_jitter_percent",
		Default: 10,
		CLIFlag: "llm-retry-jitter-percent",
		EnvVar:  "LLM_RETRY_JITTER_PERCENT",
		Type:    reflect.TypeOf(0),
		Help:    "Jitter percentage (1-100) applied to retry backoff",
	})
}

func registerLLMRetryAndLimits(registry *Registry) {
	registerLLMRetryTimingFields(registry)
	registerLLMRetryBackoffFields(registry)
	registerLLMToolLimitFields(registry)
}

// registerLLMRetryTimingFields sets invocation timeout and retry count defaults.
// It keeps the high-level retry timing knobs grouped together.
func registerLLMRetryTimingFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "llm.provider_timeout",
			Default: 5 * time.Minute,
			CLIFlag: "llm-provider-timeout",
			EnvVar:  "LLM_PROVIDER_TIMEOUT",
			Type:    durationType,
			Help:    "Maximum duration allowed for a single provider invocation before timing out",
		},
		FieldDef{
			Path:    "llm.retry_attempts",
			Default: 3,
			CLIFlag: "llm-retry-attempts",
			EnvVar:  "LLM_RETRY_ATTEMPTS",
			Type:    reflect.TypeOf(0),
			Help:    "Number of retry attempts for LLM operations",
		},
	)
}

// registerLLMRetryBackoffFields configures exponential backoff behavior.
// It includes base delay, maximum delay, and jitter toggles for retries.
func registerLLMRetryBackoffFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "llm.retry_backoff_base",
			Default: 100 * time.Millisecond,
			CLIFlag: "llm-retry-backoff-base",
			EnvVar:  "LLM_RETRY_BACKOFF_BASE",
			Type:    durationType,
			Help:    "Base delay for exponential backoff retry strategy",
		},
		FieldDef{
			Path:    "llm.retry_backoff_max",
			Default: 10 * time.Second,
			CLIFlag: "llm-retry-backoff-max",
			EnvVar:  "LLM_RETRY_BACKOFF_MAX",
			Type:    durationType,
			Help:    "Maximum delay between retry attempts",
		},
		FieldDef{
			Path:    "llm.retry_jitter",
			Default: true,
			CLIFlag: "llm-retry-jitter",
			EnvVar:  "LLM_RETRY_JITTER",
			Type:    reflect.TypeOf(true),
			Help:    "Enable random jitter in retry delays to prevent thundering herd",
		},
	)
}

// registerLLMToolLimitFields constrains downstream tool execution loops.
// It keeps concurrency and iteration safeguards in one location.
func registerLLMToolLimitFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "llm.max_concurrent_tools",
			Default: 10,
			CLIFlag: "llm-max-concurrent-tools",
			EnvVar:  "LLM_MAX_CONCURRENT_TOOLS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum number of concurrent tool executions",
		},
		FieldDef{
			Path:    "llm.max_tool_iterations",
			Default: 100,
			CLIFlag: "llm-max-tool-iterations",
			EnvVar:  "LLM_MAX_TOOL_ITERATIONS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum tool-iteration loops per request (global default)",
		},
		FieldDef{
			Path:    "llm.max_sequential_tool_errors",
			Default: 10,
			CLIFlag: "llm-max-sequential-tool-errors",
			EnvVar:  "LLM_MAX_SEQUENTIAL_TOOL_ERRORS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum sequential tool/content errors tolerated per tool before aborting",
		},
	)
}

func registerLLMRateLimiterFields(registry *Registry) {
	registerLLMRateLimiterDefaults(registry)
	registerLLMRateLimiterPerProvider(registry)
}

func registerLLMRateLimiterDefaults(registry *Registry) {
	registerLLMRateLimiterToggle(registry)
	registerLLMRateLimiterConcurrencyDefaults(registry)
	registerLLMRateLimiterThroughputDefaults(registry)
}

func registerLLMRateLimiterToggle(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.enabled",
		Default: true,
		CLIFlag: "llm-rate-limiting-enabled",
		EnvVar:  "LLM_RATE_LIMITING_ENABLED",
		Type:    reflect.TypeOf(true),
		Help:    "Enable per-provider concurrency limiting for upstream LLM calls",
	})
}

func registerLLMRateLimiterConcurrencyDefaults(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.default_concurrency",
		Default: 10,
		CLIFlag: "llm-rate-limiting-default-concurrency",
		EnvVar:  "LLM_RATE_LIMITING_DEFAULT_CONCURRENCY",
		Type:    reflect.TypeOf(0),
		Help:    "Default concurrent request limit applied when a provider override is not present",
	})
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.default_queue_size",
		Default: 100,
		CLIFlag: "llm-rate-limiting-default-queue-size",
		EnvVar:  "LLM_RATE_LIMITING_DEFAULT_QUEUE_SIZE",
		Type:    reflect.TypeOf(0),
		Help:    "Default queue size for pending requests waiting on concurrency slots",
	})
}

func registerLLMRateLimiterThroughputDefaults(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.default_requests_per_minute",
		Default: 0,
		CLIFlag: "llm-rate-limiting-default-rpm",
		EnvVar:  "LLM_RATE_LIMITING_DEFAULT_REQUESTS_PER_MINUTE",
		Type:    reflect.TypeOf(0),
		Help:    "Default request throughput cap (per minute) when provider overrides are absent",
	})
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.default_tokens_per_minute",
		Default: 0,
		CLIFlag: "llm-rate-limiting-default-tpm",
		EnvVar:  "LLM_RATE_LIMITING_DEFAULT_TOKENS_PER_MINUTE",
		Type:    reflect.TypeOf(0),
		Help:    "Default total-token throughput cap (per minute) when provider overrides are absent",
	})
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.default_request_burst",
		Default: 0,
		CLIFlag: "llm-rate-limiting-default-request-burst",
		EnvVar:  "LLM_RATE_LIMITING_DEFAULT_REQUEST_BURST",
		Type:    reflect.TypeOf(0),
		Help:    "Burst size applied to request-per-minute limiters when provider overrides are absent",
	})
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.default_token_burst",
		Default: 0,
		CLIFlag: "llm-rate-limiting-default-token-burst",
		EnvVar:  "LLM_RATE_LIMITING_DEFAULT_TOKEN_BURST",
		Type:    reflect.TypeOf(0),
		Help:    "Burst size applied to token-per-minute limiters when provider overrides are absent",
	})
}

func registerLLMRateLimiterPerProvider(registry *Registry) {
	defaults := map[string]map[string]int{
		"cerebras":   makeProviderLimits(10, 100, 60, 0, 0, 0),
		"groq":       makeProviderLimits(10, 100, 60, 0, 0, 0),
		"openai":     makeProviderLimits(50, 200, 120, 0, 0, 0),
		"anthropic":  makeProviderLimits(20, 150, 60, 0, 0, 0),
		"google":     makeProviderLimits(30, 150, 90, 0, 0, 0),
		"ollama":     makeProviderLimits(10, 100, 60, 0, 0, 0),
		"openrouter": makeProviderLimits(10, 100, 60, 0, 0, 0),
	}
	registry.Register(&FieldDef{
		Path:    "llm.rate_limiting.per_provider_limits",
		Default: defaults,
		EnvVar:  "LLM_RATE_LIMITING_PER_PROVIDER_LIMITS",
		Type:    reflect.TypeOf(map[string]map[string]int{}),
		Help:    "Per-provider concurrency and queue overrides keyed by provider name",
	})
}

//nolint:unparam // tokensPerMinute reserved for future per-provider overrides
func makeProviderLimits(
	concurrency, queueSize, requestsPerMinute, tokensPerMinute, requestBurst, tokenBurst int,
) map[string]int {
	return map[string]int{
		"concurrency":         concurrency,
		"queue_size":          queueSize,
		"requests_per_minute": requestsPerMinute,
		"tokens_per_minute":   tokensPerMinute,
		"request_burst":       requestBurst,
		"token_burst":         tokenBurst,
	}
}

func registerLLMConversationControls(registry *Registry) {
	registerLLMProgressControls(registry)
	registerLLMRestartControls(registry)
	registerLLMCompactionControls(registry)
	registerLLMTelemetryControls(registry)
	registerLLMToolCaps(registry)
	registerLLMOutputValidation(registry)
}

func registerLLMProgressControls(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "llm.max_consecutive_successes",
			Default: 3,
			CLIFlag: "llm-max-consecutive-successes",
			EnvVar:  "LLM_MAX_CONSECUTIVE_SUCCESSES",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum successful tool calls allowed without progress before halting the loop",
		},
		FieldDef{
			Path:    "llm.enable_progress_tracking",
			Default: false,
			CLIFlag: "llm-enable-progress-tracking",
			EnvVar:  "LLM_ENABLE_PROGRESS_TRACKING",
			Type:    reflect.TypeOf(true),
			Help:    "Enable loop progress tracking to detect stalled agent iterations",
		},
		FieldDef{
			Path:    "llm.no_progress_threshold",
			Default: 3,
			CLIFlag: "llm-no-progress-threshold",
			EnvVar:  "LLM_NO_PROGRESS_THRESHOLD",
			Type:    reflect.TypeOf(0),
			Help:    "Number of consecutive iterations without progress tolerated before aborting",
		},
		FieldDef{
			Path:    "llm.enable_dynamic_prompt_state",
			Default: false,
			CLIFlag: "llm-enable-dynamic-prompt-state",
			EnvVar:  "LLM_ENABLE_DYNAMIC_PROMPT_STATE",
			Type:    reflect.TypeOf(true),
			Help:    "Include dynamic loop diagnostics (usage, budgets) in the system prompt each turn",
		},
	)
}

func registerLLMToolCaps(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "llm.tool_call_caps.default",
			Default: 0,
			EnvVar:  "LLM_TOOL_CALL_CAPS_DEFAULT",
			Type:    reflect.TypeOf(0),
			Help:    "Default per-tool invocation cap enforced during a single orchestrator run",
		},
		FieldDef{
			Path:    "llm.tool_call_caps.overrides",
			Default: map[string]int{},
			Type:    reflect.TypeOf(map[string]int{}),
			Help:    "Per-tool overrides for invocation caps (map of tool -> limit)",
		},
	)
}

func registerLLMOutputValidation(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "llm.finalize_output_retries",
			Default: 0,
			CLIFlag: "llm-finalize-output-retries",
			EnvVar:  "LLM_FINALIZE_OUTPUT_RETRIES",
			Type:    reflect.TypeOf(0),
			Help:    "Number of retries allowed when validating the final structured output",
		},
		FieldDef{
			Path:    "llm.structured_output_retries",
			Default: 2,
			CLIFlag: "llm-structured-output-retries",
			EnvVar:  "LLM_STRUCTURED_OUTPUT_RETRIES",
			Type:    reflect.TypeOf(0),
			Help:    "Retry budget for intermediate structured output validation before falling back or failing",
		},
	)
}

func registerLLMRestartControls(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.enable_loop_restarts",
		Default: false,
		CLIFlag: "llm-enable-loop-restarts",
		EnvVar:  "LLM_ENABLE_LOOP_RESTARTS",
		Type:    reflect.TypeOf(true),
		Help:    "Enable automatic loop restarts when progress remains stalled",
	})
	registry.Register(&FieldDef{
		Path:    "llm.restart_stall_threshold",
		Default: 2,
		CLIFlag: "llm-restart-stall-threshold",
		EnvVar:  "LLM_RESTART_STALL_THRESHOLD",
		Type:    reflect.TypeOf(0),
		Help:    "Number of stalled iterations required before triggering a loop restart",
	})
	registry.Register(&FieldDef{
		Path:    "llm.max_loop_restarts",
		Default: 0,
		CLIFlag: "llm-max-loop-restarts",
		EnvVar:  "LLM_MAX_LOOP_RESTARTS",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum loop restarts allowed per orchestration run (0 disables)",
	})
}

func registerLLMCompactionControls(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.enable_context_compaction",
		Default: false,
		CLIFlag: "llm-enable-context-compaction",
		EnvVar:  "LLM_ENABLE_CONTEXT_COMPACTION",
		Type:    reflect.TypeOf(true),
		Help:    "Enable summarisation-based context compaction when usage exceeds thresholds",
	})
	registry.Register(&FieldDef{
		Path:    "llm.context_compaction_threshold",
		Default: 0.85,
		CLIFlag: "llm-context-compaction-threshold",
		EnvVar:  "LLM_CONTEXT_COMPACTION_THRESHOLD",
		Type:    reflect.TypeOf(0.0),
		Help:    "Context usage ratio (0-1) that triggers compaction when enabled",
	})
	registry.Register(&FieldDef{
		Path:    "llm.context_compaction_cooldown",
		Default: 2,
		CLIFlag: "llm-context-compaction-cooldown",
		EnvVar:  "LLM_CONTEXT_COMPACTION_COOLDOWN",
		Type:    reflect.TypeOf(0),
		Help:    "Number of loop iterations to wait between compaction attempts (0 disables)",
	})
}

func registerLLMTelemetryControls(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.context_warning_thresholds",
		Default: []float64{0.7, 0.85},
		CLIFlag: "llm-context-warning-thresholds",
		EnvVar:  "LLM_CONTEXT_WARNING_THRESHOLDS",
		Type:    reflect.TypeOf([]float64{}),
		Help:    "Comma-separated context usage ratios (0-1) that trigger telemetry warnings",
	})
	registry.Register(&FieldDef{
		Path:    "llm.usage_metrics.persist_buckets",
		Default: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		Type:    reflect.TypeOf([]float64{}),
		Help:    "Histogram bucket boundaries (seconds) for usage persistence latency metrics",
	})
}

// registerLLMDefaultParams registers global default LLM parameters
func registerLLMDefaultParams(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "llm.default_top_p",
		Default: 0.0,
		CLIFlag: "llm-default-top-p",
		EnvVar:  "LLM_DEFAULT_TOP_P",
		Type:    reflect.TypeOf(0.0),
		Help:    "Default TopP (nucleus sampling) for LLM requests (0 = use provider default)",
	})
	registry.Register(&FieldDef{
		Path:    "llm.default_frequency_penalty",
		Default: 0.0,
		CLIFlag: "llm-default-frequency-penalty",
		EnvVar:  "LLM_DEFAULT_FREQUENCY_PENALTY",
		Type:    reflect.TypeOf(0.0),
		Help:    "Default frequency penalty to reduce repetition (0 = no penalty)",
	})
	registry.Register(&FieldDef{
		Path:    "llm.default_presence_penalty",
		Default: 0.0,
		CLIFlag: "llm-default-presence-penalty",
		EnvVar:  "LLM_DEFAULT_PRESENCE_PENALTY",
		Type:    reflect.TypeOf(0.0),
		Help:    "Default presence penalty to encourage diversity (0 = no penalty)",
	})
	registry.Register(&FieldDef{
		Path:    "llm.default_seed",
		Default: 0,
		CLIFlag: "llm-default-seed",
		EnvVar:  "LLM_DEFAULT_SEED",
		Type:    reflect.TypeOf(0),
		Help:    "Default seed for reproducible outputs (0 = disabled/non-deterministic)",
	})
}

func registerRateLimitFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "ratelimit.global_rate.limit",
			Default: int64(100),
			EnvVar:  "RATELIMIT_GLOBAL_LIMIT",
			Type:    reflect.TypeOf(int64(0)),
			Help:    "Global rate limit (requests per period)",
		},
		FieldDef{
			Path:    "ratelimit.global_rate.period",
			Default: 1 * time.Minute,
			EnvVar:  "RATELIMIT_GLOBAL_PERIOD",
			Type:    durationType,
			Help:    "Global rate limit period",
		},
		FieldDef{
			Path:    "ratelimit.api_key_rate.limit",
			Default: int64(100),
			EnvVar:  "RATELIMIT_API_KEY_LIMIT",
			Type:    reflect.TypeOf(int64(0)),
			Help:    "API key rate limit (requests per period)",
		},
		FieldDef{
			Path:    "ratelimit.api_key_rate.period",
			Default: 1 * time.Minute,
			EnvVar:  "RATELIMIT_API_KEY_PERIOD",
			Type:    durationType,
			Help:    "API key rate limit period",
		},
		FieldDef{
			Path:    "ratelimit.prefix",
			Default: "compozy:ratelimit:",
			EnvVar:  "RATELIMIT_PREFIX",
			Type:    reflect.TypeOf(""),
			Help:    "Key prefix for rate limit storage",
		},
		FieldDef{
			Path:    "ratelimit.max_retry",
			Default: 3,
			EnvVar:  "RATELIMIT_MAX_RETRY",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum retries for rate limit operations",
		},
	)
}

func registerCLIFields(registry *Registry) {
	registerBasicCLIFields(registry)
	registerOutputFormatFields(registry)
	registerBehaviorFlags(registry)
}

func registerRedisFields(registry *Registry) {
	registerRedisConnectionFields(registry)
	registerRedisPoolFields(registry)
	registerRedisTimeoutFields(registry)
	registerRedisRetryFields(registry)
	registerRedisTLSFields(registry)
}

func registerRedisConnectionFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "redis.url",
		Default: "",
		CLIFlag: "",
		EnvVar:  "REDIS_URL",
		Type:    reflect.TypeOf(""),
		Help:    "Redis connection URL (takes precedence over individual parameters)",
	})
	registry.Register(&FieldDef{
		Path:    "redis.host",
		Default: "localhost",
		CLIFlag: "",
		EnvVar:  "REDIS_HOST",
		Type:    reflect.TypeOf(""),
		Help:    "Redis server hostname",
	})
	registry.Register(&FieldDef{
		Path:    "redis.port",
		Default: "6379",
		CLIFlag: "",
		EnvVar:  "REDIS_PORT",
		Type:    reflect.TypeOf(""),
		Help:    "Redis server port",
	})
	registry.Register(&FieldDef{
		Path:    "redis.password",
		Default: "",
		CLIFlag: "",
		EnvVar:  "REDIS_PASSWORD",
		Type:    reflect.TypeOf(""),
		Help:    "Redis password",
	})
	registry.Register(&FieldDef{
		Path:    "redis.db",
		Default: 0,
		CLIFlag: "",
		EnvVar:  "REDIS_DB",
		Type:    reflect.TypeOf(0),
		Help:    "Redis database number",
	})
}

func registerRedisPoolFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "redis.pool_size",
		Default: 10,
		CLIFlag: "",
		EnvVar:  "REDIS_POOL_SIZE",
		Type:    reflect.TypeOf(0),
		Help:    "Connection pool size",
	})
	registry.Register(&FieldDef{
		Path:    "redis.min_idle_conns",
		Default: 0,
		CLIFlag: "",
		EnvVar:  "REDIS_MIN_IDLE_CONNS",
		Type:    reflect.TypeOf(0),
		Help:    "Minimum number of idle connections",
	})
	registry.Register(&FieldDef{
		Path:    "redis.max_idle_conns",
		Default: 0,
		CLIFlag: "",
		EnvVar:  "REDIS_MAX_IDLE_CONNS",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum number of idle connections",
	})
	registry.Register(&FieldDef{
		Path:    "redis.notification_buffer_size",
		Default: 100,
		CLIFlag: "",
		EnvVar:  "REDIS_NOTIFICATION_BUFFER_SIZE",
		Type:    reflect.TypeOf(0),
		Help:    "Buffer size for pub/sub notifications",
	})
}

func registerRedisTimeoutFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "redis.dial_timeout",
		Default: 5 * time.Second,
		CLIFlag: "",
		EnvVar:  "REDIS_DIAL_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for establishing new connections",
	})
	registry.Register(&FieldDef{
		Path:    "redis.read_timeout",
		Default: 3 * time.Second,
		CLIFlag: "",
		EnvVar:  "REDIS_READ_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for socket reads",
	})
	registry.Register(&FieldDef{
		Path:    "redis.write_timeout",
		Default: 3 * time.Second,
		CLIFlag: "",
		EnvVar:  "REDIS_WRITE_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for socket writes",
	})
	registry.Register(&FieldDef{
		Path:    "redis.pool_timeout",
		Default: 4 * time.Second,
		CLIFlag: "",
		EnvVar:  "REDIS_POOL_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for getting connection from pool",
	})
	registry.Register(&FieldDef{
		Path:    "redis.ping_timeout",
		Default: 1 * time.Second,
		CLIFlag: "",
		EnvVar:  "REDIS_PING_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for ping command",
	})
}

func registerRedisRetryFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "redis.max_retries",
		Default: 3,
		CLIFlag: "",
		EnvVar:  "REDIS_MAX_RETRIES",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum number of retries",
	})
	registry.Register(&FieldDef{
		Path:    "redis.min_retry_backoff",
		Default: 8 * time.Millisecond,
		CLIFlag: "",
		EnvVar:  "REDIS_MIN_RETRY_BACKOFF",
		Type:    durationType,
		Help:    "Minimum backoff between retries",
	})
	registry.Register(&FieldDef{
		Path:    "redis.max_retry_backoff",
		Default: 512 * time.Millisecond,
		CLIFlag: "",
		EnvVar:  "REDIS_MAX_RETRY_BACKOFF",
		Type:    durationType,
		Help:    "Maximum backoff between retries",
	})
}

func registerRedisTLSFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "redis.tls_enabled",
		Default: false,
		CLIFlag: "",
		EnvVar:  "REDIS_TLS_ENABLED",
		Type:    reflect.TypeOf(true),
		Help:    "Enable TLS encryption",
	})
}

func registerBasicCLIFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "cli.api_key",
		Default: "",
		CLIFlag: "api-key",
		EnvVar:  "COMPOZY_API_KEY",
		Type:    reflect.TypeOf(""),
		Help:    "Compozy API key for authentication",
	})
	registry.Register(&FieldDef{
		Path:    "cli.base_url",
		Default: "http://localhost:5001",
		CLIFlag: "base-url",
		EnvVar:  "COMPOZY_BASE_URL",
		Type:    reflect.TypeOf(""),
		Help:    "Base URL for Compozy API",
	})
	registry.Register(&FieldDef{
		Path:    "cli.timeout",
		Default: 30 * time.Second,
		CLIFlag: "timeout",
		EnvVar:  "COMPOZY_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for API requests",
	})
	registry.Register(&FieldDef{
		Path:    "cli.max_retries",
		Default: 3,
		CLIFlag: "max-retries",
		EnvVar:  "COMPOZY_MAX_RETRIES",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum retry attempts for CLI HTTP requests",
	})
	registry.Register(&FieldDef{
		Path:    "cli.mode",
		Default: "auto",
		CLIFlag: "mode",
		EnvVar:  "COMPOZY_MODE",
		Type:    reflect.TypeOf(""),
		Help:    "CLI mode: auto, json, or tui",
	})
}

func registerOutputFormatFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:      "cli.default_format",
		Default:   "tui",
		CLIFlag:   "format",
		Shorthand: "f",
		EnvVar:    "COMPOZY_DEFAULT_FORMAT",
		Type:      reflect.TypeOf(""),
		Help:      "Default output format: json, tui, or auto",
	})
	registry.Register(&FieldDef{
		Path:    "cli.color_mode",
		Default: "auto",
		CLIFlag: "color-mode",
		EnvVar:  "COMPOZY_COLOR_MODE",
		Type:    reflect.TypeOf(""),
		Help:    "Color mode: auto, on, or off",
	})
	registry.Register(&FieldDef{
		Path:    "cli.page_size",
		Default: 50,
		CLIFlag: "page-size",
		EnvVar:  "COMPOZY_PAGE_SIZE",
		Type:    reflect.TypeOf(0),
		Help:    "Default page size for paginated results",
	})
	registry.Register(&FieldDef{
		Path:      "cli.output_format_alias",
		Default:   "",
		CLIFlag:   "output",
		Shorthand: "o",
		EnvVar:    "",
		Type:      reflect.TypeOf(""),
		Help:      "Output format alias (same as --format)",
	})
	registry.Register(&FieldDef{
		Path:    "cli.no_color",
		Default: false,
		CLIFlag: "no-color",
		EnvVar:  "",
		Type:    reflect.TypeOf(true),
		Help:    "Disable color output",
	})
}

func registerBehaviorFlags(registry *Registry) {
	registerCLIModeFlags(registry)
	registerCLIPathFlags(registry)
	registerCLIPortReleaseFlags(registry)
	registerCLIDevWatcherFields(registry)
	registerCLIUsersFields(registry)
}

// registerCLIModeFlags configures CLI verbosity and interaction toggles.
// It bundles runtime-facing flags like debug, quiet, and interactive mode.
func registerCLIModeFlags(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "cli.debug",
			Default: false,
			CLIFlag: "debug",
			EnvVar:  "COMPOZY_DEBUG",
			Type:    reflect.TypeOf(true),
			Help:    "Enable debug output and verbose logging",
		},
		FieldDef{
			Path:      "cli.quiet",
			Default:   false,
			CLIFlag:   "quiet",
			Shorthand: "q",
			EnvVar:    "COMPOZY_QUIET",
			Type:      reflect.TypeOf(true),
			Help:      "Suppress non-essential output for automation and scripting",
		},
		FieldDef{
			Path:    "cli.interactive",
			Default: false,
			CLIFlag: "interactive",
			EnvVar:  "COMPOZY_INTERACTIVE",
			Type:    reflect.TypeOf(true),
			Help:    "Force interactive mode even when CI or non-TTY detected",
		},
	)
}

// registerCLIPathFlags manages filesystem-related CLI inputs.
// These fields capture config, working directory, and env file controls.
func registerCLIPathFlags(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:      "cli.config_file",
			Default:   "",
			CLIFlag:   "config",
			Shorthand: "c",
			EnvVar:    "COMPOZY_CONFIG_FILE",
			Type:      reflect.TypeOf(""),
			Help:      "Path to configuration file",
		},
		FieldDef{
			Path:    "cli.cwd",
			Default: "",
			CLIFlag: "cwd",
			EnvVar:  "COMPOZY_CWD",
			Type:    reflect.TypeOf(""),
			Help:    "Working directory for the project",
		},
		FieldDef{
			Path:    "cli.env_file",
			Default: "",
			CLIFlag: "env-file",
			EnvVar:  "COMPOZY_ENV_FILE",
			Type:    reflect.TypeOf(""),
			Help:    "Path to the environment variables file",
		},
	)
}

// registerCLIPortReleaseFlags tunes how CLI utilities wait on ports.
// It separates timing knobs for better readability within the registry.
func registerCLIPortReleaseFlags(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "cli.port_release_timeout",
			Default: 5 * time.Second,
			CLIFlag: "port-release-timeout",
			EnvVar:  "COMPOZY_PORT_RELEASE_TIMEOUT",
			Type:    durationType,
			Help:    "Maximum time to wait for a port to become available",
		},
		FieldDef{
			Path:    "cli.port_release_poll_interval",
			Default: 100 * time.Millisecond,
			CLIFlag: "port-release-poll-interval",
			EnvVar:  "COMPOZY_PORT_RELEASE_POLL_INTERVAL",
			Type:    durationType,
			Help:    "How often to check if a port has become available",
		},
		FieldDef{
			Path:    "cli.file_watch_interval",
			Default: 1 * time.Second,
			CLIFlag: "file-watch-interval",
			EnvVar:  "COMPOZY_FILE_WATCH_INTERVAL",
			Type:    durationType,
			Help:    "Polling interval for CLI file watching fallback when fsnotify support is unavailable",
		},
	)
}

func registerCLIDevWatcherFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "cli.dev.watcher_debounce",
			Default: 200 * time.Millisecond,
			EnvVar:  "COMPOZY_DEV_WATCHER_DEBOUNCE",
			Type:    durationType,
			Help:    "Quiet period before restarting the dev server after file changes",
		},
		FieldDef{
			Path:    "cli.dev.watcher_retry_initial",
			Default: 500 * time.Millisecond,
			EnvVar:  "COMPOZY_DEV_WATCHER_RETRY_INITIAL",
			Type:    durationType,
			Help:    "Initial delay before retrying a failed dev server restart",
		},
		FieldDef{
			Path:    "cli.dev.watcher_retry_max",
			Default: 30 * time.Second,
			EnvVar:  "COMPOZY_DEV_WATCHER_RETRY_MAX",
			Type:    durationType,
			Help:    "Maximum backoff between dev server restart attempts",
		},
	)
}

func registerCLIUsersFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "cli.users.active_window_days",
			Default: 30,
			CLIFlag: "cli-users-active-window-days",
			EnvVar:  "COMPOZY_USERS_ACTIVE_WINDOW_DAYS",
			Type:    reflect.TypeOf(0),
			Help:    "Number of days defining an 'active' user for CLI filtering",
		},
	)
}

func registerCacheFields(registry *Registry) {
	registerCacheDataFields(registry)
	registerCacheCompressionFields(registry)
}

func registerCacheDataFields(registry *Registry) {
	registerCacheToggleFields(registry)
	registerCacheKeyFields(registry)
	registerCacheRuntimeFields(registry)
}

// registerCacheToggleFields toggles cache availability and TTL policies.
// It isolates the high-level enablement and retention configuration.
func registerCacheToggleFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "cache.enabled",
			Default: true,
			EnvVar:  "CACHE_ENABLED",
			Type:    reflect.TypeOf(false),
			Help:    "Enable or disable caching functionality",
		},
		FieldDef{
			Path:    "cache.ttl",
			Default: 24 * time.Hour,
			EnvVar:  "CACHE_TTL",
			Type:    durationType,
			Help:    "Default TTL for cached data",
		},
	)
}

// registerCacheKeyFields configures cache namespacing and eviction.
// Grouping these fields highlights how entries are organized and purged.
func registerCacheKeyFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "cache.prefix",
			Default: "compozy:cache:",
			EnvVar:  "CACHE_PREFIX",
			Type:    reflect.TypeOf(""),
			Help:    "Key prefix for all cache entries",
		},
		FieldDef{
			Path:    "cache.eviction_policy",
			Default: "lru",
			EnvVar:  "CACHE_EVICTION_POLICY",
			Type:    reflect.TypeOf(""),
			Help:    "Cache eviction policy (lru, lfu, ttl)",
		},
	)
}

// registerCacheRuntimeFields handles cache sizing and monitoring knobs.
// It keeps item size, stats sampling, and key scanning parameters together.
func registerCacheRuntimeFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "cache.max_item_size",
			Default: int64(1_048_576),
			EnvVar:  "CACHE_MAX_ITEM_SIZE",
			Type:    reflect.TypeOf(int64(0)),
			Help:    "Maximum size for cached items in bytes",
		},
		FieldDef{
			Path:    "cache.stats_interval",
			Default: 5 * time.Minute,
			EnvVar:  "CACHE_STATS_INTERVAL",
			Type:    durationType,
			Help:    "Interval for logging cache statistics (0 to disable)",
		},
		FieldDef{
			Path:    "cache.key_scan_count",
			Default: 100,
			EnvVar:  "CACHE_KEY_SCAN_COUNT",
			Type:    reflect.TypeOf(int(0)),
			Help:    "COUNT hint used by Redis SCAN for key iteration (positive integer)",
		},
	)
}

func registerCacheCompressionFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "cache.compression_enabled",
		Default: true,
		CLIFlag: "",
		EnvVar:  "CACHE_COMPRESSION_ENABLED",
		Type:    reflect.TypeOf(false),
		Help:    "Enable compression for large cache values",
	})
	registry.Register(&FieldDef{
		Path:    "cache.compression_threshold",
		Default: int64(1024), // 1KB
		CLIFlag: "",
		EnvVar:  "CACHE_COMPRESSION_THRESHOLD",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Minimum size in bytes to trigger compression",
	})
}

func registerWorkerFields(registry *Registry) {
	registerWorkerCacheLifecycleFields(registry)
	registerWorkerDispatcherFields(registry)
	registerWorkerTemporalFields(registry)
	registerWorkerConcurrencyFields(registry)
	registerWorkerActivityDefaults(registry)
	registerWorkerErrorHandlerFields(registry)
}

func registerWorkerCacheLifecycleFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "worker.config_store_ttl",
		Default: 24 * time.Hour,
		CLIFlag: "",
		EnvVar:  "WORKER_CONFIG_STORE_TTL",
		Type:    durationType,
		Help:    "TTL for configuration data in cache",
	})
	registry.Register(&FieldDef{
		Path:    "worker.heartbeat_cleanup_timeout",
		Default: 5 * time.Second,
		CLIFlag: "",
		EnvVar:  "WORKER_HEARTBEAT_CLEANUP_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for cleaning up dispatcher heartbeats",
	})
	registry.Register(&FieldDef{
		Path:    "worker.mcp_shutdown_timeout",
		Default: 30 * time.Second,
		CLIFlag: "",
		EnvVar:  "WORKER_MCP_SHUTDOWN_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for MCP server shutdown",
	})
	registry.Register(&FieldDef{
		Path:    "worker.mcp_proxy_health_check_timeout",
		Default: 10 * time.Second,
		CLIFlag: "",
		EnvVar:  "WORKER_MCP_PROXY_HEALTH_CHECK_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for MCP proxy health checks",
	})
}

func registerWorkerDispatcherFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "worker.dispatcher.heartbeat_ttl",
		Default: 5 * time.Minute,
		CLIFlag: "",
		EnvVar:  "WORKER_DISPATCHER_HEARTBEAT_TTL",
		Type:    durationType,
		Help:    "TTL for dispatcher heartbeat records",
	})
	registry.Register(&FieldDef{
		Path:    "worker.dispatcher.stale_threshold",
		Default: 2 * time.Minute,
		CLIFlag: "",
		EnvVar:  "WORKER_DISPATCHER_STALE_THRESHOLD",
		Type:    durationType,
		Help:    "Duration after which a dispatcher heartbeat is considered stale",
	})
	registry.Register(&FieldDef{
		Path:    "worker.dispatcher_retry_delay",
		Default: 50 * time.Millisecond,
		CLIFlag: "",
		EnvVar:  "WORKER_DISPATCHER_RETRY_DELAY",
		Type:    durationType,
		Help:    "Delay between dispatcher retry attempts",
	})
	registry.Register(&FieldDef{
		Path:    "worker.dispatcher_max_retries",
		Default: 2,
		CLIFlag: "",
		EnvVar:  "WORKER_DISPATCHER_MAX_RETRIES",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum number of dispatcher startup retries",
	})
}

func registerWorkerTemporalFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "worker.start_workflow_timeout",
		Default: 5 * time.Second,
		CLIFlag: "",
		EnvVar:  "WORKER_START_WORKFLOW_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for starting a workflow execution to avoid hanging requests",
	})
}

func registerWorkerConcurrencyFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "worker.max_concurrent_activity_execution_size",
		Default: 0,
		CLIFlag: "",
		EnvVar:  "WORKER_MAX_CONCURRENT_ACTIVITIES",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum concurrent activity executions (0 = auto based on CPU)",
	})
	registry.Register(&FieldDef{
		Path:    "worker.max_concurrent_workflow_execution_size",
		Default: 0,
		CLIFlag: "",
		EnvVar:  "WORKER_MAX_CONCURRENT_WORKFLOWS",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum concurrent workflow task executions (0 = auto based on CPU)",
	})
	registry.Register(&FieldDef{
		Path:    "worker.max_concurrent_local_activity_execution_size",
		Default: 0,
		CLIFlag: "",
		EnvVar:  "WORKER_MAX_CONCURRENT_LOCAL_ACTIVITIES",
		Type:    reflect.TypeOf(0),
		Help:    "Maximum concurrent local activity executions (0 = auto based on CPU)",
	})
}

func registerWorkerActivityDefaults(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "worker.activity_start_to_close_timeout",
		Default: 5 * time.Minute,
		CLIFlag: "",
		EnvVar:  "WORKER_ACTIVITY_START_TO_CLOSE_TIMEOUT",
		Type:    durationType,
		Help:    "Default activity start-to-close timeout",
	})
	registry.Register(&FieldDef{
		Path:    "worker.activity_heartbeat_timeout",
		Default: 30 * time.Second,
		CLIFlag: "",
		EnvVar:  "WORKER_ACTIVITY_HEARTBEAT_TIMEOUT",
		Type:    durationType,
		Help:    "Default activity heartbeat timeout",
	})
	registry.Register(&FieldDef{
		Path:    "worker.activity_max_retries",
		Default: 3,
		CLIFlag: "",
		EnvVar:  "WORKER_ACTIVITY_MAX_RETRIES",
		Type:    reflect.TypeOf(0),
		Help:    "Default maximum activity retry attempts",
	})
}

func registerWorkerErrorHandlerFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "worker.error_handler_timeout",
		Default: 30 * time.Second,
		CLIFlag: "",
		EnvVar:  "WORKER_ERROR_HANDLER_TIMEOUT",
		Type:    durationType,
		Help:    "Timeout for workflow error handler activities",
	})
	registry.Register(&FieldDef{
		Path:    "worker.error_handler_max_retries",
		Default: 3,
		CLIFlag: "",
		EnvVar:  "WORKER_ERROR_HANDLER_MAX_RETRIES",
		Type:    reflect.TypeOf(0),
		Help:    "Retry attempts for workflow error handler activities",
	})
}

func registerMCPProxyFields(registry *Registry) {
	registerMCPProxyServerFields(registry)
	registerMCPProxyTimeoutFields(registry)
	registerMCPProxyPoolFields(registry)
}

// registerMCPProxyServerFields configures core MCP proxy endpoint details.
// It isolates mode, host, port, and base URL from pooling concerns.
func registerMCPProxyServerFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "mcp_proxy.mode",
			Default: "standalone",
			EnvVar:  "MCP_PROXY_MODE",
			Type:    reflect.TypeOf(""),
			Help:    "MCP proxy mode: 'standalone' embeds the proxy (needs fixed port); empty keeps external proxy defaults",
		},
		FieldDef{
			Path:    "mcp_proxy.host",
			Default: "127.0.0.1",
			CLIFlag: "mcp-host",
			EnvVar:  "MCP_PROXY_HOST",
			Type:    reflect.TypeOf(""),
			Help:    "Host interface for MCP proxy server to bind to",
		},
		FieldDef{
			Path:    "mcp_proxy.port",
			Default: 0,
			CLIFlag: "mcp-port",
			EnvVar:  "MCP_PROXY_PORT",
			Type:    reflect.TypeOf(0),
			Help:    "Port for MCP proxy server to listen on (0 = ephemeral)",
		},
		FieldDef{
			Path:    "mcp_proxy.base_url",
			Default: "",
			CLIFlag: "mcp-base-url",
			EnvVar:  "MCP_PROXY_BASE_URL",
			Type:    reflect.TypeOf(""),
			Help:    "Base URL for MCP proxy server (auto-generated if empty)",
		},
	)
}

// registerMCPProxyTimeoutFields defines shutdown and idle timeouts.
// It keeps timing controls clear alongside other helper functions.
func registerMCPProxyTimeoutFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "mcp_proxy.shutdown_timeout",
			Default: 10 * time.Second,
			EnvVar:  "MCP_PROXY_SHUTDOWN_TIMEOUT",
			Type:    durationType,
			Help:    "Maximum time to wait for graceful shutdown",
		},
		FieldDef{
			Path:    "mcp_proxy.idle_conn_timeout",
			Default: 90 * time.Second,
			EnvVar:  "MCP_PROXY_IDLE_CONN_TIMEOUT",
			Type:    durationType,
			Help:    "Maximum time an idle connection is kept before closing",
		},
	)
}

// registerMCPProxyPoolFields configures HTTP connection pool capacities.
// It isolates concurrency-related fields for easier auditing.
func registerMCPProxyPoolFields(registry *Registry) {
	registerFieldDefs(
		registry,
		FieldDef{
			Path:    "mcp_proxy.max_idle_conns",
			Default: 128,
			EnvVar:  "MCP_PROXY_MAX_IDLE_CONNS",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum number of idle connections kept globally",
		},
		FieldDef{
			Path:    "mcp_proxy.max_idle_conns_per_host",
			Default: 128,
			EnvVar:  "MCP_PROXY_MAX_IDLE_CONNS_PER_HOST",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum idle connections maintained per proxy host",
		},
		FieldDef{
			Path:    "mcp_proxy.max_conns_per_host",
			Default: 128,
			EnvVar:  "MCP_PROXY_MAX_CONNS_PER_HOST",
			Type:    reflect.TypeOf(0),
			Help:    "Maximum total concurrent connections allowed per proxy host",
		},
	)
}

func registerWebhooksFields(registry *Registry) {
	registry.Register(&FieldDef{
		Path:    "webhooks.default_method",
		Default: "POST",
		CLIFlag: "webhook-default-method",
		EnvVar:  "WEBHOOKS_DEFAULT_METHOD",
		Type:    reflect.TypeOf(""),
		Help:    "Default HTTP method for webhook requests",
	})
	registry.Register(&FieldDef{
		Path:    "webhooks.default_max_body",
		Default: int64(1 << 20), // 1MB
		CLIFlag: "webhook-default-max-body",
		EnvVar:  "WEBHOOKS_DEFAULT_MAX_BODY",
		Type:    reflect.TypeOf(int64(0)),
		Help:    "Default maximum body size for webhook requests (bytes)",
	})
	registry.Register(&FieldDef{
		Path:    "webhooks.default_dedupe_ttl",
		Default: 10 * time.Minute,
		CLIFlag: "webhook-default-dedupe-ttl",
		EnvVar:  "WEBHOOKS_DEFAULT_DEDUPE_TTL",
		Type:    durationType,
		Help:    "Default time-to-live for webhook deduplication",
	})
	registry.Register(&FieldDef{
		Path:    "webhooks.stripe_skew",
		Default: 5 * time.Minute,
		CLIFlag: "webhook-stripe-skew",
		EnvVar:  "WEBHOOKS_STRIPE_SKEW",
		Type:    durationType,
		Help:    "Allowed timestamp skew for Stripe webhook verification",
	})
}
