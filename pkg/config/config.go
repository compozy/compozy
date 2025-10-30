package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/config/definition"
	"github.com/mitchellh/mapstructure"
)

const (
	databaseDriverPostgres = "postgres"
	databaseDriverSQLite   = "sqlite"
)

func isEmbeddedMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case ModeMemory, ModePersistent:
		return true
	default:
		return false
	}
}

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
	// Mode controls the global deployment model.
	//
	// "memory" (default): In-memory SQLite with embedded services for tests, CI pipelines, and quick prototypes.
	// "persistent": File-backed SQLite with embedded services for local development that needs state between runs.
	// "distributed": PostgreSQL with external Temporal/Redis for production-grade deployments.
	Mode string `koanf:"mode"   env:"COMPOZY_MODE" json:"mode"   yaml:"mode"   mapstructure:"mode"   validate:"omitempty"`
	// Server configures the HTTP API server settings.
	//
	// $ref: schema://application#server
	Server ServerConfig `koanf:"server"                    json:"server" yaml:"server" mapstructure:"server" validate:"required"`

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

	// Stream configures real-time streaming defaults.
	Stream StreamConfig `koanf:"stream" json:"stream" yaml:"stream" mapstructure:"stream"`

	// Tasks configures task execution tunables.
	Tasks TasksConfig `koanf:"tasks" json:"tasks" yaml:"tasks" mapstructure:"tasks"`

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

	// Knowledge configures default behaviors for knowledge ingestion and retrieval.
	Knowledge KnowledgeConfig `koanf:"knowledge" json:"knowledge" yaml:"knowledge" mapstructure:"knowledge"`

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

	// Attachments configures global attachment handling limits and policies.
	//
	// $ref: schema://application#attachments
	Attachments AttachmentsConfig `koanf:"attachments" json:"attachments" yaml:"attachments" mapstructure:"attachments"`

	// Webhooks configures webhook processing and validation settings.
	//
	// $ref: schema://application#webhooks
	Webhooks WebhooksConfig `koanf:"webhooks" json:"webhooks" yaml:"webhooks" mapstructure:"webhooks"`
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

	// Timeouts contains operator-tunable timeouts and retry/backoff settings
	// for server startup, shutdown, and HTTP behavior.
	//
	// $ref: schema://application#server.timeouts
	Timeouts ServerTimeouts `koanf:"timeouts" json:"timeouts" yaml:"timeouts" mapstructure:"timeouts"`

	// SourceOfTruth selects where the server loads workflows from.
	//
	// Values:
	//   - "repo": load YAML from repository and index to store (default)
	//   - "builder": load workflows from ResourceStore (compile-from-store)
	SourceOfTruth string `koanf:"source_of_truth" json:"source_of_truth" yaml:"source_of_truth" mapstructure:"source_of_truth" env:"SERVER_SOURCE_OF_TRUTH" validate:"oneof=repo builder"`

	// SeedFromRepoOnEmpty controls whether builder mode seeds the store from
	// repository YAML once when the store is empty. Disabled by default.
	SeedFromRepoOnEmpty bool `koanf:"seed_from_repo_on_empty" json:"seed_from_repo_on_empty" yaml:"seed_from_repo_on_empty" mapstructure:"seed_from_repo_on_empty" env:"SERVER_SEED_FROM_REPO_ON_EMPTY"`

	// Reconciler configures the workflow reconciler subsystem.
	Reconciler ReconcilerConfig `koanf:"reconciler" json:"reconciler" yaml:"reconciler" mapstructure:"reconciler"`
}

// ServerTimeouts defines tunable durations for server operations.
type ServerTimeouts struct {
	MonitoringInit           time.Duration `koanf:"monitoring_init"                json:"monitoring_init"                yaml:"monitoring_init"                mapstructure:"monitoring_init"`
	MonitoringShutdown       time.Duration `koanf:"monitoring_shutdown"            json:"monitoring_shutdown"            yaml:"monitoring_shutdown"            mapstructure:"monitoring_shutdown"`
	DBShutdown               time.Duration `koanf:"db_shutdown"                    json:"db_shutdown"                    yaml:"db_shutdown"                    mapstructure:"db_shutdown"`
	WorkerShutdown           time.Duration `koanf:"worker_shutdown"                json:"worker_shutdown"                yaml:"worker_shutdown"                mapstructure:"worker_shutdown"`
	ServerShutdown           time.Duration `koanf:"server_shutdown"                json:"server_shutdown"                yaml:"server_shutdown"                mapstructure:"server_shutdown"`
	HTTPRead                 time.Duration `koanf:"http_read"                      json:"http_read"                      yaml:"http_read"                      mapstructure:"http_read"`
	HTTPWrite                time.Duration `koanf:"http_write"                     json:"http_write"                     yaml:"http_write"                     mapstructure:"http_write"`
	HTTPIdle                 time.Duration `koanf:"http_idle"                      json:"http_idle"                      yaml:"http_idle"                      mapstructure:"http_idle"`
	HTTPReadHeader           time.Duration `koanf:"http_read_header"               json:"http_read_header"               yaml:"http_read_header"               mapstructure:"http_read_header"`
	ScheduleRetryMaxDuration time.Duration `koanf:"schedule_retry_max_duration"    json:"schedule_retry_max_duration"    yaml:"schedule_retry_max_duration"    mapstructure:"schedule_retry_max_duration"`
	ScheduleRetryBaseDelay   time.Duration `koanf:"schedule_retry_base_delay"      json:"schedule_retry_base_delay"      yaml:"schedule_retry_base_delay"      mapstructure:"schedule_retry_base_delay"`
	ScheduleRetryMaxDelay    time.Duration `koanf:"schedule_retry_max_delay"       json:"schedule_retry_max_delay"       yaml:"schedule_retry_max_delay"       mapstructure:"schedule_retry_max_delay"`
	// ScheduleRetryMaxAttempts limits the number of reconciliation retry attempts.
	// When set to a value >= 1, this takes precedence over ScheduleRetryMaxDuration.
	ScheduleRetryMaxAttempts int `koanf:"schedule_retry_max_attempts"    json:"schedule_retry_max_attempts"    yaml:"schedule_retry_max_attempts"    mapstructure:"schedule_retry_max_attempts"`
	// ScheduleRetryBackoffSeconds sets the base backoff in seconds used to build
	// the exponential backoff for reconciliation retries. If zero, a sensible
	// default is applied in code.
	ScheduleRetryBackoffSeconds int `koanf:"schedule_retry_backoff_seconds" json:"schedule_retry_backoff_seconds" yaml:"schedule_retry_backoff_seconds" mapstructure:"schedule_retry_backoff_seconds"`
	// KnowledgeIngest bounds startup knowledge ingestion executions.
	// Applies to ingestKnowledgeBasesOnStart. Zero disables the timeout.
	KnowledgeIngest      time.Duration `koanf:"knowledge_ingest"               json:"knowledge_ingest"               yaml:"knowledge_ingest"               mapstructure:"knowledge_ingest"               env:"SERVER_KNOWLEDGE_INGEST_TIMEOUT"`
	TemporalReachability time.Duration `koanf:"temporal_reachability"          json:"temporal_reachability"          yaml:"temporal_reachability"          mapstructure:"temporal_reachability"`
	StartProbeDelay      time.Duration `koanf:"start_probe_delay"              json:"start_probe_delay"              yaml:"start_probe_delay"              mapstructure:"start_probe_delay"`
}

// ReconcilerConfig defines tunable options for the workflow reconciler.
type ReconcilerConfig struct {
	QueueCapacity   int           `koanf:"queue_capacity"    json:"queue_capacity"    yaml:"queue_capacity"    mapstructure:"queue_capacity"    env:"SERVER_RECONCILER_QUEUE_CAPACITY"    validate:"min=0"`
	DebounceWait    time.Duration `koanf:"debounce_wait"     json:"debounce_wait"     yaml:"debounce_wait"     mapstructure:"debounce_wait"     env:"SERVER_RECONCILER_DEBOUNCE_WAIT"     validate:"min=0"`
	DebounceMaxWait time.Duration `koanf:"debounce_max_wait" json:"debounce_max_wait" yaml:"debounce_max_wait" mapstructure:"debounce_max_wait" env:"SERVER_RECONCILER_DEBOUNCE_MAX_WAIT" validate:"min=0"`
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

	// APIKeyLastUsedMaxConcurrency bounds background workers that stamp API key last-used timestamps.
	//
	// Default: 10. Set to 0 to disable asynchronous updates.
	APIKeyLastUsedMaxConcurrency int `koanf:"api_key_last_used_max_concurrency" json:"api_key_last_used_max_concurrency" yaml:"api_key_last_used_max_concurrency" mapstructure:"api_key_last_used_max_concurrency" env:"SERVER_AUTH_API_KEY_LAST_USED_MAX_CONCURRENCY" validate:"min=0"`

	// APIKeyLastUsedTimeout limits how long asynchronous last-used updates may run before timing out.
	//
	// Default: 2s.
	APIKeyLastUsedTimeout time.Duration `koanf:"api_key_last_used_timeout" json:"api_key_last_used_timeout" yaml:"api_key_last_used_timeout" mapstructure:"api_key_last_used_timeout" env:"SERVER_AUTH_API_KEY_LAST_USED_TIMEOUT"`
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
	// Driver selects the backing database driver implementation.
	//
	// Supported drivers:
	//   - "postgres": default, full production deployment
	//   - "sqlite": lightweight single-node deployments
	//
	// Defaults to "postgres" when omitted for backward compatibility.
	Driver string `koanf:"driver" json:"driver" yaml:"driver" mapstructure:"driver" env:"DB_DRIVER" validate:"omitempty,oneof=postgres sqlite"`

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
	Password string `koanf:"password" json:"password" yaml:"password" mapstructure:"password" env:"DB_PASSWORD" sensitive:"true"`

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

	// MigrationTimeout sets the maximum allowed time for applying database
	// migrations during startup. It must be equal to or greater than the
	// advisory lock acquisition window used by ApplyMigrationsWithLock (45s).
	//
	// Default: 2m
	MigrationTimeout time.Duration `koanf:"migration_timeout" json:"migration_timeout" yaml:"migration_timeout" mapstructure:"migration_timeout" env:"DB_MIGRATION_TIMEOUT"`

	// MaxOpenConns caps total simultaneous PostgreSQL connections from this service.
	//
	// Default: `25`
	MaxOpenConns int `koanf:"max_open_conns" json:"max_open_conns" yaml:"max_open_conns" mapstructure:"max_open_conns" env:"DB_MAX_OPEN_CONNS"`

	// MaxIdleConns defines the number of connections kept idle in the pool.
	//
	// Default: `5`
	MaxIdleConns int `koanf:"max_idle_conns" json:"max_idle_conns" yaml:"max_idle_conns" mapstructure:"max_idle_conns" env:"DB_MAX_IDLE_CONNS"`

	// Path specifies the SQLite database location or ":memory:".
	//
	// Values:
	//   - ":memory:" for ephemeral in-memory databases
	//   - Relative or absolute file path for persistent storage
	Path string `koanf:"path" json:"path" yaml:"path" mapstructure:"path" env:"DB_PATH"`

	// BusyTimeout configures SQLite PRAGMA busy_timeout for lock contention.
	// When unset, a sensible default is applied by the SQLite provider.
	BusyTimeout time.Duration `koanf:"busy_timeout" json:"busy_timeout" yaml:"busy_timeout" mapstructure:"busy_timeout" env:"DB_BUSY_TIMEOUT"`

	// ConnMaxLifetime bounds how long a connection may be reused.
	//
	// Default: `5m`
	ConnMaxLifetime time.Duration `koanf:"conn_max_lifetime" json:"conn_max_lifetime" yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`

	// ConnMaxIdleTime bounds how long an idle connection is retained before recycling.
	//
	// Default: `1m`
	ConnMaxIdleTime time.Duration `koanf:"conn_max_idle_time" json:"conn_max_idle_time" yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME"`

	// PingTimeout bounds how long connectivity checks may wait when establishing the pool.
	//
	// Default: `3s`
	PingTimeout time.Duration `koanf:"ping_timeout" json:"ping_timeout" yaml:"ping_timeout" mapstructure:"ping_timeout" env:"DB_PING_TIMEOUT"`

	// HealthCheckTimeout limits the runtime health check duration before reporting failure.
	//
	// Default: `1s`
	HealthCheckTimeout time.Duration `koanf:"health_check_timeout" json:"health_check_timeout" yaml:"health_check_timeout" mapstructure:"health_check_timeout" env:"DB_HEALTH_CHECK_TIMEOUT"`

	// HealthCheckPeriod configures how frequently the pool performs background health checks.
	//
	// Default: `30s`
	HealthCheckPeriod time.Duration `koanf:"health_check_period" json:"health_check_period" yaml:"health_check_period" mapstructure:"health_check_period" env:"DB_HEALTH_CHECK_PERIOD"`

	// ConnectTimeout bounds how long the driver may spend establishing new PostgreSQL connections.
	//
	// Default: `5s`
	ConnectTimeout time.Duration `koanf:"connect_timeout" json:"connect_timeout" yaml:"connect_timeout" mapstructure:"connect_timeout" env:"DB_CONNECT_TIMEOUT"`
}

// Validate ensures driver-specific requirements are satisfied and normalizes configuration.
func (c *DatabaseConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("database config is required")
	}
	driver := strings.ToLower(strings.TrimSpace(c.Driver))
	if driver == "" {
		driver = databaseDriverPostgres
	}
	c.Driver = driver
	switch driver {
	case databaseDriverPostgres:
		return c.validatePostgres()
	case databaseDriverSQLite:
		return c.validateSQLite()
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}
}

func (c *DatabaseConfig) validatePostgres() error {
	if strings.TrimSpace(c.ConnString) != "" {
		return nil
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("postgres driver requires host or conn_string")
	}
	if strings.TrimSpace(c.Port) == "" ||
		strings.TrimSpace(c.User) == "" ||
		strings.TrimSpace(c.DBName) == "" {
		return fmt.Errorf("postgres driver requires host, port, user, and name or conn_string")
	}
	return nil
}

func (c *DatabaseConfig) validateSQLite() error {
	path := strings.TrimSpace(c.Path)
	if path == "" {
		return fmt.Errorf("sqlite driver requires path")
	}
	normalized, err := normalizeSQLitePath(path)
	if err != nil {
		return err
	}
	c.Path = normalized
	return nil
}

func normalizeSQLitePath(input string) (string, error) {
	if input == ":memory:" || strings.HasPrefix(input, "file::memory:") {
		return input, nil
	}
	if strings.Contains(input, "\x00") {
		return "", fmt.Errorf("sqlite path contains null byte")
	}
	if strings.ContainsRune(input, '\n') || strings.ContainsRune(input, '\r') {
		return "", fmt.Errorf("sqlite path cannot contain newlines")
	}
	if strings.HasPrefix(input, "file:") {
		return validateAndNormalizeFileDSN(input)
	}
	cleaned := filepath.Clean(input)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("sqlite path must reference a file, got %q", input)
	}
	if hasParentTraversal(cleaned) {
		return "", fmt.Errorf("sqlite path cannot traverse directories: %q", input)
	}
	return cleaned, nil
}

func validateAndNormalizeFileDSN(input string) (string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("invalid sqlite file DSN: %w", err)
	}
	clean := filepath.Clean(strings.TrimPrefix(u.Path, "//"))
	if clean == "." || clean == string(filepath.Separator) {
		return "", fmt.Errorf("sqlite path must reference a file, got %q", input)
	}
	if hasParentTraversal(clean) {
		return "", fmt.Errorf("sqlite path cannot traverse directories: %q", input)
	}
	for key := range u.Query() {
		if !isAllowedSQLiteParam(key) {
			return "", fmt.Errorf("unsupported sqlite DSN parameter: %s", key)
		}
	}
	u.Path = clean
	return u.String(), nil
}

func isAllowedSQLiteParam(key string) bool {
	switch strings.ToLower(key) {
	case "mode", "cache", "_pragma":
		return true
	default:
		return false
	}
}

func hasParentTraversal(path string) bool {
	segments := strings.Split(path, string(filepath.Separator))
	for _, segment := range segments {
		if segment == ".." {
			return true
		}
	}
	return false
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
	// Mode controls how the application connects to Temporal.
	//
	// Values:
	//   - "memory": Launch embedded Temporal with in-memory persistence for the fastest feedback loops (default)
	//   - "persistent": Launch embedded Temporal with file-backed persistence for stateful local development
	//   - "distributed": Connect to an external Temporal deployment for production workloads
	Mode string `koanf:"mode" env:"TEMPORAL_MODE" json:"mode" yaml:"mode" mapstructure:"mode" validate:"omitempty"`

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

	// Standalone configures the embedded Temporal server used by memory and persistent modes.
	Standalone StandaloneConfig `koanf:"standalone" env_prefix:"TEMPORAL_STANDALONE" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
}

// StandaloneConfig configures the embedded Temporal server that powers memory and persistent modes.
//
// These options mirror the embedded server configuration so users can manage development
// and test environments without touching production settings.
type StandaloneConfig struct {
	// DatabaseFile specifies the SQLite database location.
	//
	// Use ":memory:" for ephemeral storage or provide a file path for persistence.
	DatabaseFile string `koanf:"database_file" env:"TEMPORAL_STANDALONE_DATABASE_FILE" json:"database_file" yaml:"database_file" mapstructure:"database_file"`

	// FrontendPort sets the gRPC port for the Temporal frontend service.
	FrontendPort int `koanf:"frontend_port" env:"TEMPORAL_STANDALONE_FRONTEND_PORT" json:"frontend_port" yaml:"frontend_port" mapstructure:"frontend_port"`

	// BindIP determines the IP address Temporal services bind to.
	BindIP string `koanf:"bind_ip" env:"TEMPORAL_STANDALONE_BIND_IP" json:"bind_ip" yaml:"bind_ip" mapstructure:"bind_ip"`

	// Namespace specifies the default namespace created on startup.
	Namespace string `koanf:"namespace" env:"TEMPORAL_STANDALONE_NAMESPACE" json:"namespace" yaml:"namespace" mapstructure:"namespace"`

	// ClusterName customizes the Temporal cluster name for embedded deployments.
	ClusterName string `koanf:"cluster_name" env:"TEMPORAL_STANDALONE_CLUSTER_NAME" json:"cluster_name" yaml:"cluster_name" mapstructure:"cluster_name"`

	// EnableUI toggles the Temporal Web UI server.
	EnableUI bool `koanf:"enable_ui" env:"TEMPORAL_STANDALONE_ENABLE_UI" json:"enable_ui" yaml:"enable_ui" mapstructure:"enable_ui"`

	// RequireUI enforces UI availability; startup fails when UI cannot be launched.
	RequireUI bool `koanf:"require_ui" env:"TEMPORAL_STANDALONE_REQUIRE_UI" json:"require_ui" yaml:"require_ui" mapstructure:"require_ui"`

	// UIPort sets the HTTP port for the Temporal Web UI.
	UIPort int `koanf:"ui_port" env:"TEMPORAL_STANDALONE_UI_PORT" json:"ui_port" yaml:"ui_port" mapstructure:"ui_port"`

	// LogLevel controls Temporal server logging verbosity.
	LogLevel string `koanf:"log_level" env:"TEMPORAL_STANDALONE_LOG_LEVEL" json:"log_level" yaml:"log_level" mapstructure:"log_level"`

	// StartTimeout defines the maximum startup wait duration.
	StartTimeout time.Duration `koanf:"start_timeout" env:"TEMPORAL_STANDALONE_START_TIMEOUT" json:"start_timeout" yaml:"start_timeout" mapstructure:"start_timeout"`
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

	// TaskExecutionTimeoutDefault controls the fallback timeout applied when direct task executions
	// omit a timeout value. Applies to synchronous and asynchronous direct executions triggered via API.
	// Default: 60s
	TaskExecutionTimeoutDefault time.Duration `koanf:"task_execution_timeout_default" env:"TASK_EXECUTION_TIMEOUT_DEFAULT" json:"task_execution_timeout_default" yaml:"task_execution_timeout_default" mapstructure:"task_execution_timeout_default"`

	// TaskExecutionTimeoutMax caps the maximum timeout allowed for direct task executions.
	// Client-specified or configuration-derived timeouts exceeding this value are rejected.
	// Default: 300s
	TaskExecutionTimeoutMax time.Duration `koanf:"task_execution_timeout_max" env:"TASK_EXECUTION_TIMEOUT_MAX" json:"task_execution_timeout_max" yaml:"task_execution_timeout_max" mapstructure:"task_execution_timeout_max"`

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
	// Leave empty to disable custom TypeScript tools and rely on built-in capabilities.
	EntrypointPath string `koanf:"entrypoint_path" env:"RUNTIME_ENTRYPOINT_PATH" json:"entrypoint_path" yaml:"entrypoint_path" mapstructure:"entrypoint_path"`

	// BunPermissions defines runtime security permissions for Bun.
	//
	// Default: ["--allow-read"]
	BunPermissions []string `koanf:"bun_permissions" env:"RUNTIME_BUN_PERMISSIONS" json:"bun_permissions" yaml:"bun_permissions" mapstructure:"bun_permissions"`

	// NativeTools configures native cp__ tool behavior and guards.
	NativeTools NativeToolsConfig `koanf:"native_tools" json:"native_tools" yaml:"native_tools" mapstructure:"native_tools"`
}

// StreamConfig holds defaults for streaming endpoints.
type StreamConfig struct {
	Agent    AgentStreamConfig        `koanf:"agent"    json:"agent"    yaml:"agent"    mapstructure:"agent"`
	Task     TaskStreamEndpointConfig `koanf:"task"     json:"task"     yaml:"task"     mapstructure:"task"`
	LLM      LLMStreamConfig          `koanf:"llm"      json:"llm"      yaml:"llm"      mapstructure:"llm"`
	Workflow WorkflowStreamConfig     `koanf:"workflow" json:"workflow" yaml:"workflow" mapstructure:"workflow"`
}

// AgentStreamConfig defines tunables for agent execution streaming.
type AgentStreamConfig struct {
	DefaultPoll        time.Duration `koanf:"default_poll"        env:"STREAM_AGENT_DEFAULT_POLL"        json:"default_poll"        yaml:"default_poll"        mapstructure:"default_poll"        validate:"min=0"`
	MinPoll            time.Duration `koanf:"min_poll"            env:"STREAM_AGENT_MIN_POLL"            json:"min_poll"            yaml:"min_poll"            mapstructure:"min_poll"            validate:"min=0"`
	MaxPoll            time.Duration `koanf:"max_poll"            env:"STREAM_AGENT_MAX_POLL"            json:"max_poll"            yaml:"max_poll"            mapstructure:"max_poll"            validate:"min=0"`
	HeartbeatFrequency time.Duration `koanf:"heartbeat_frequency" env:"STREAM_AGENT_HEARTBEAT_FREQUENCY" json:"heartbeat_frequency" yaml:"heartbeat_frequency" mapstructure:"heartbeat_frequency" validate:"min=0"`
	ReplayLimit        int           `koanf:"replay_limit"        env:"STREAM_AGENT_REPLAY_LIMIT"        json:"replay_limit"        yaml:"replay_limit"        mapstructure:"replay_limit"        validate:"min=0"`
}

// TaskStreamEndpointConfig defines tunables for task streaming endpoints.
type TaskStreamEndpointConfig struct {
	DefaultPoll        time.Duration        `koanf:"default_poll"         env:"STREAM_TASK_DEFAULT_POLL"         json:"default_poll"         yaml:"default_poll"         mapstructure:"default_poll"         validate:"min=0"`
	MinPoll            time.Duration        `koanf:"min_poll"             env:"STREAM_TASK_MIN_POLL"             json:"min_poll"             yaml:"min_poll"             mapstructure:"min_poll"             validate:"min=0"`
	MaxPoll            time.Duration        `koanf:"max_poll"             env:"STREAM_TASK_MAX_POLL"             json:"max_poll"             yaml:"max_poll"             mapstructure:"max_poll"             validate:"min=0"`
	HeartbeatFrequency time.Duration        `koanf:"heartbeat_frequency"  env:"STREAM_TASK_HEARTBEAT_FREQUENCY"  json:"heartbeat_frequency"  yaml:"heartbeat_frequency"  mapstructure:"heartbeat_frequency"  validate:"min=0"`
	RedisChannelPrefix string               `koanf:"redis_channel_prefix" env:"STREAM_TASK_REDIS_CHANNEL_PREFIX" json:"redis_channel_prefix" yaml:"redis_channel_prefix" mapstructure:"redis_channel_prefix"`
	RedisLogPrefix     string               `koanf:"redis_log_prefix"     env:"STREAM_TASK_REDIS_LOG_PREFIX"     json:"redis_log_prefix"     yaml:"redis_log_prefix"     mapstructure:"redis_log_prefix"`
	RedisSeqPrefix     string               `koanf:"redis_seq_prefix"     env:"STREAM_TASK_REDIS_SEQ_PREFIX"     json:"redis_seq_prefix"     yaml:"redis_seq_prefix"     mapstructure:"redis_seq_prefix"`
	RedisMaxEntries    int64                `koanf:"redis_max_entries"    env:"STREAM_TASK_REDIS_MAX_ENTRIES"    json:"redis_max_entries"    yaml:"redis_max_entries"    mapstructure:"redis_max_entries"    validate:"min=0"`
	RedisTTL           time.Duration        `koanf:"redis_ttl"            env:"STREAM_TASK_REDIS_TTL"            json:"redis_ttl"            yaml:"redis_ttl"            mapstructure:"redis_ttl"`
	ReplayLimit        int                  `koanf:"replay_limit"         env:"STREAM_TASK_REPLAY_LIMIT"         json:"replay_limit"         yaml:"replay_limit"         mapstructure:"replay_limit"         validate:"min=0"`
	Text               TaskTextStreamConfig `koanf:"text"                                                        json:"text"                 yaml:"text"                 mapstructure:"text"`
}

// LLMStreamConfig defines tunables for LLM fallback streaming behavior.
type LLMStreamConfig struct {
	FallbackSegmentLimit int `koanf:"fallback_segment_limit" env:"STREAM_LLM_FALLBACK_SEGMENT_LIMIT" json:"fallback_segment_limit" yaml:"fallback_segment_limit" mapstructure:"fallback_segment_limit" validate:"min=0"`
}

// TaskTextStreamConfig defines tunables for plain-text task streaming.
type TaskTextStreamConfig struct {
	MaxSegmentRunes int           `koanf:"max_segment_runes" env:"STREAM_TASK_TEXT_MAX_SEGMENT_RUNES" json:"max_segment_runes" yaml:"max_segment_runes" mapstructure:"max_segment_runes" validate:"min=0"`
	PublishTimeout  time.Duration `koanf:"publish_timeout"   env:"STREAM_TASK_TEXT_PUBLISH_TIMEOUT"   json:"publish_timeout"   yaml:"publish_timeout"   mapstructure:"publish_timeout"   validate:"min=0"`
}

// WorkflowStreamConfig defines tunables for workflow execution streaming.
type WorkflowStreamConfig struct {
	DefaultPoll        time.Duration `koanf:"default_poll"        env:"STREAM_WORKFLOW_DEFAULT_POLL"        json:"default_poll"        yaml:"default_poll"        mapstructure:"default_poll"        validate:"min=0"`
	MinPoll            time.Duration `koanf:"min_poll"            env:"STREAM_WORKFLOW_MIN_POLL"            json:"min_poll"            yaml:"min_poll"            mapstructure:"min_poll"            validate:"min=0"`
	MaxPoll            time.Duration `koanf:"max_poll"            env:"STREAM_WORKFLOW_MAX_POLL"            json:"max_poll"            yaml:"max_poll"            mapstructure:"max_poll"            validate:"min=0"`
	HeartbeatFrequency time.Duration `koanf:"heartbeat_frequency" env:"STREAM_WORKFLOW_HEARTBEAT_FREQUENCY" json:"heartbeat_frequency" yaml:"heartbeat_frequency" mapstructure:"heartbeat_frequency" validate:"min=0"`
	QueryTimeout       time.Duration `koanf:"query_timeout"       env:"STREAM_WORKFLOW_QUERY_TIMEOUT"       json:"query_timeout"       yaml:"query_timeout"       mapstructure:"query_timeout"       validate:"min=0"`
}

// TasksConfig aggregates task execution tunables.
type TasksConfig struct {
	Retry  TaskRetryConfig  `koanf:"retry"  json:"retry"  yaml:"retry"  mapstructure:"retry"`
	Wait   TaskWaitConfig   `koanf:"wait"   json:"wait"   yaml:"wait"   mapstructure:"wait"`
	Stream TaskStreamConfig `koanf:"stream" json:"stream" yaml:"stream" mapstructure:"stream"`
}

// TaskRetryConfig captures retry behavior for dependent lookups.
type TaskRetryConfig struct {
	ChildState TaskChildStateRetryConfig `koanf:"child_state" json:"child_state" yaml:"child_state" mapstructure:"child_state"`
}

// TaskChildStateRetryConfig defines retry strategy for child state lookups.
type TaskChildStateRetryConfig struct {
	MaxAttempts int           `koanf:"max_attempts" env:"TASKS_RETRY_CHILD_MAX_ATTEMPTS" json:"max_attempts" yaml:"max_attempts" mapstructure:"max_attempts" validate:"min=1"`
	BaseBackoff time.Duration `koanf:"base_backoff" env:"TASKS_RETRY_CHILD_BASE_BACKOFF" json:"base_backoff" yaml:"base_backoff" mapstructure:"base_backoff" validate:"min=0"`
}

// TaskWaitConfig captures sibling wait tunables.
type TaskWaitConfig struct {
	Siblings TaskSiblingWaitConfig `koanf:"siblings" json:"siblings" yaml:"siblings" mapstructure:"siblings"`
}

// TaskSiblingWaitConfig tunes sibling polling behavior.
type TaskSiblingWaitConfig struct {
	PollInterval time.Duration `koanf:"poll_interval" env:"TASKS_WAIT_SIBLINGS_POLL_INTERVAL" json:"poll_interval" yaml:"poll_interval" mapstructure:"poll_interval" validate:"min=0"`
	Timeout      time.Duration `koanf:"timeout"       env:"TASKS_WAIT_SIBLINGS_TIMEOUT"       json:"timeout"       yaml:"timeout"       mapstructure:"timeout"       validate:"min=0"`
}

// TaskStreamConfig limits stream chunk publication.
type TaskStreamConfig struct {
	MaxChunks int `koanf:"max_chunks" env:"TASKS_STREAM_MAX_CHUNKS" json:"max_chunks" yaml:"max_chunks" mapstructure:"max_chunks"`
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

	// MaxConfigFileNestingDepth limits nesting depth when parsing configuration files.
	//
	// Prevents stack overflow from deeply nested YAML documents supplied by users.
	// Default: 100
	MaxConfigFileNestingDepth int `koanf:"max_config_file_nesting_depth" validate:"min=1" env:"LIMITS_MAX_CONFIG_FILE_NESTING_DEPTH" json:"max_config_file_nesting_depth" yaml:"max_config_file_nesting_depth" mapstructure:"max_config_file_nesting_depth"`

	// MaxStringLength limits individual string values.
	//
	// Applies to all string fields in requests and responses.
	// Default: 10MB (10485760 bytes)
	MaxStringLength int `koanf:"max_string_length" validate:"min=1" env:"LIMITS_MAX_STRING_LENGTH" json:"max_string_length" yaml:"max_string_length" mapstructure:"max_string_length"`

	// MaxConfigFileSize limits configuration file size during loads.
	//
	// Prevents memory exhaustion when loading large YAML/JSON documents.
	// Default: 10MB (10485760 bytes)
	MaxConfigFileSize int `koanf:"max_config_file_size" validate:"min=1" env:"LIMITS_MAX_CONFIG_FILE_SIZE" json:"max_config_file_size" yaml:"max_config_file_size" mapstructure:"max_config_file_size"`

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

// KnowledgeConfig contains default tuning knobs for knowledge ingestion and retrieval.
//
// These defaults backfill optional fields in project/workflow knowledge definitions
// so operators can adjust behavior globally without editing every YAML file.
type KnowledgeConfig struct {
	// EmbedderBatchSize defines the default batch size used when projecting documents.
	//
	// Applies when an embedder configuration omits config.batch_size.
	// Larger batches improve throughput but may increase provider throttling risk.
	EmbedderBatchSize int `koanf:"embedder_batch_size" env:"KNOWLEDGE_EMBEDDER_BATCH_SIZE" json:"embedder_batch_size" yaml:"embedder_batch_size" mapstructure:"embedder_batch_size" validate:"min=1"`

	// ChunkSize sets the default chunk size (in tokens) for source splitting.
	//
	// Enforced when chunking.size is not provided.
	// Must remain within the supported range for knowledge ingestion.
	ChunkSize int `koanf:"chunk_size" env:"KNOWLEDGE_CHUNK_SIZE" json:"chunk_size" yaml:"chunk_size" mapstructure:"chunk_size" validate:"min=64,max=8192"`

	// ChunkOverlap defines the default overlap between adjacent chunks.
	//
	// Applied when chunking.overlap is omitted. Values greater than or equal to ChunkSize
	// are ignored during normalization.
	ChunkOverlap int `koanf:"chunk_overlap" env:"KNOWLEDGE_CHUNK_OVERLAP" json:"chunk_overlap" yaml:"chunk_overlap" mapstructure:"chunk_overlap" validate:"min=0"`

	// RetrievalTopK specifies the default number of results returned during retrieval.
	//
	// Used when retrieval.top_k is unset on a knowledge binding or base definition.
	RetrievalTopK int `koanf:"retrieval_top_k" env:"KNOWLEDGE_RETRIEVAL_TOP_K" json:"retrieval_top_k" yaml:"retrieval_top_k" mapstructure:"retrieval_top_k" validate:"min=1,max=50"`

	// RetrievalMinScore sets the default minimum similarity score required for matches.
	//
	// Applied when retrieval.min_score is not provided. Must fall within [0.0, 1.0].
	RetrievalMinScore float64 `koanf:"retrieval_min_score" env:"KNOWLEDGE_RETRIEVAL_MIN_SCORE" json:"retrieval_min_score" yaml:"retrieval_min_score" mapstructure:"retrieval_min_score" validate:"min=0,max=1"`

	// MaxMarkdownFileSizeBytes limits the size of markdown files ingested from disk or URLs.
	//
	// Files exceeding this threshold are rejected during ingestion.
	MaxMarkdownFileSizeBytes int `koanf:"max_markdown_file_size_bytes" env:"KNOWLEDGE_MAX_MARKDOWN_FILE_SIZE_BYTES" json:"max_markdown_file_size_bytes" yaml:"max_markdown_file_size_bytes" mapstructure:"max_markdown_file_size_bytes" validate:"min=1024"`

	// VectorHTTPTimeout bounds HTTP requests made by knowledge vector backends.
	//
	// Applies to HTTP-based vector stores such as Qdrant.
	VectorHTTPTimeout time.Duration `koanf:"vector_http_timeout" env:"KNOWLEDGE_VECTOR_HTTP_TIMEOUT" json:"vector_http_timeout" yaml:"vector_http_timeout" mapstructure:"vector_http_timeout" validate:"min=0"`

	// VectorDBs declares global vector database connections available to knowledge features.
	// When SQLite is selected, at least one external vector database should be configured.
	VectorDBs []VectorDBConfig `koanf:"vector_dbs" json:"vector_dbs" yaml:"vector_dbs" mapstructure:"vector_dbs"`
}

// VectorDBConfig describes an external vector database integration available at runtime.
// It captures connection parameters so knowledge ingestion and retrieval can target the store.
type VectorDBConfig struct {
	ID       string         `koanf:"id"       json:"id"       yaml:"id"       mapstructure:"id"`
	Provider string         `koanf:"provider" json:"provider" yaml:"provider" mapstructure:"provider"`
	URL      string         `koanf:"url"      json:"url"      yaml:"url"      mapstructure:"url"`
	Path     string         `koanf:"path"     json:"path"     yaml:"path"     mapstructure:"path"`
	Options  map[string]any `koanf:"options"  json:"options"  yaml:"options"  mapstructure:"options"`
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

	// RateLimiting configures concurrency throttles and provider queues.
	//
	// The limiter guards upstream APIs from bursty traffic and honors Retry-After headers.
	// Defaults ensure conservative limits that can be tuned per provider.
	RateLimiting LLMRateLimitConfig `koanf:"rate_limiting" json:"rate_limiting" yaml:"rate_limiting" mapstructure:"rate_limiting"`

	// ProviderTimeout sets the maximum duration allowed for a single provider invocation.
	//
	// Applies to each GenerateContent call (including retries) to keep the orchestrator responsive.
	// Default: 5m
	ProviderTimeout time.Duration `koanf:"provider_timeout" env:"LLM_PROVIDER_TIMEOUT" json:"provider_timeout" yaml:"provider_timeout" mapstructure:"provider_timeout"`

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

	// MaxConsecutiveSuccesses controls how many consecutive successful tool calls
	// without progress are tolerated before the orchestrator halts the loop.
	// Default: 3 (orchestrator default)
	MaxConsecutiveSuccesses int `koanf:"max_consecutive_successes" env:"LLM_MAX_CONSECUTIVE_SUCCESSES" json:"max_consecutive_successes" yaml:"max_consecutive_successes" mapstructure:"max_consecutive_successes" validate:"min=0"`

	// EnableProgressTracking toggles loop progress tracking to detect stalled
	// conversations or repeated tool usage without advancement.
	// Default: false
	EnableProgressTracking bool `koanf:"enable_progress_tracking" env:"LLM_ENABLE_PROGRESS_TRACKING" json:"enable_progress_tracking" yaml:"enable_progress_tracking" mapstructure:"enable_progress_tracking"`

	// NoProgressThreshold configures how many loop iterations without progress are
	// tolerated before the orchestrator considers the interaction stalled.
	// Default: 3 (orchestrator default)
	NoProgressThreshold int `koanf:"no_progress_threshold" env:"LLM_NO_PROGRESS_THRESHOLD" json:"no_progress_threshold" yaml:"no_progress_threshold" mapstructure:"no_progress_threshold" validate:"min=0"`

	// EnableLoopRestarts toggles whether the orchestrator may restart the loop when no progress is detected.
	EnableLoopRestarts bool `koanf:"enable_loop_restarts" env:"LLM_ENABLE_LOOP_RESTARTS" json:"enable_loop_restarts" yaml:"enable_loop_restarts" mapstructure:"enable_loop_restarts"`

	// RestartStallThreshold controls how many stalled iterations trigger a loop restart.
	RestartStallThreshold int `koanf:"restart_stall_threshold" env:"LLM_RESTART_STALL_THRESHOLD" json:"restart_stall_threshold" yaml:"restart_stall_threshold" mapstructure:"restart_stall_threshold" validate:"min=0"`

	// MaxLoopRestarts caps how many restarts are attempted per loop execution.
	MaxLoopRestarts int `koanf:"max_loop_restarts" env:"LLM_MAX_LOOP_RESTARTS" json:"max_loop_restarts" yaml:"max_loop_restarts" mapstructure:"max_loop_restarts" validate:"min=0"`

	// EnableContextCompaction toggles summary-based compaction when context usage nears limits.
	EnableContextCompaction bool `koanf:"enable_context_compaction" env:"LLM_ENABLE_CONTEXT_COMPACTION" json:"enable_context_compaction" yaml:"enable_context_compaction" mapstructure:"enable_context_compaction"`

	// ContextCompactionThreshold expresses the context usage ratio (0-1) that triggers compaction.
	ContextCompactionThreshold float64 `koanf:"context_compaction_threshold" env:"LLM_CONTEXT_COMPACTION_THRESHOLD" json:"context_compaction_threshold" yaml:"context_compaction_threshold" mapstructure:"context_compaction_threshold" validate:"min=0"`

	// ContextCompactionCooldown specifies how many loop iterations to wait between compaction attempts.
	ContextCompactionCooldown int `koanf:"context_compaction_cooldown" env:"LLM_CONTEXT_COMPACTION_COOLDOWN" json:"context_compaction_cooldown" yaml:"context_compaction_cooldown" mapstructure:"context_compaction_cooldown" validate:"min=0"`

	// EnableDynamicPromptState toggles inclusion of orchestrator loop state inside the system prompt.
	EnableDynamicPromptState bool `koanf:"enable_dynamic_prompt_state" env:"LLM_ENABLE_DYNAMIC_PROMPT_STATE" json:"enable_dynamic_prompt_state" yaml:"enable_dynamic_prompt_state" mapstructure:"enable_dynamic_prompt_state"`

	// ToolCallCaps configures per-tool invocation caps enforced during orchestration.
	ToolCallCaps ToolCallCapsConfig `koanf:"tool_call_caps" json:"tool_call_caps" yaml:"tool_call_caps" mapstructure:"tool_call_caps"`

	// StructuredOutputRetryAttempts controls how many validation retries are attempted before failing.
	//
	// When set to 0, the orchestrator default (2) is used.
	StructuredOutputRetryAttempts int `koanf:"structured_output_retries" env:"LLM_STRUCTURED_OUTPUT_RETRIES" json:"structured_output_retries" yaml:"structured_output_retries" mapstructure:"structured_output_retries" validate:"min=0"`

	// FinalizeOutputRetryAttempts overrides the number of retries allowed when final structured output is invalid.
	FinalizeOutputRetryAttempts int `koanf:"finalize_output_retries" env:"LLM_FINALIZE_OUTPUT_RETRIES" json:"finalize_output_retries" yaml:"finalize_output_retries" mapstructure:"finalize_output_retries" validate:"min=0"`

	// AllowedMCPNames restricts which MCP servers/tools are considered eligible
	// for advertisement and invocation. When empty, all discovered MCP tools
	// are eligible.
	AllowedMCPNames []string `koanf:"allowed_mcp_names" env:"LLM_ALLOWED_MCP_NAMES" json:"allowed_mcp_names" yaml:"allowed_mcp_names" mapstructure:"allowed_mcp_names"`

	// DeniedMCPNames excludes MCP servers/tools from advertisement and invocation.
	// Entries take precedence over the allow list.
	DeniedMCPNames []string `koanf:"denied_mcp_names" env:"LLM_DENIED_MCP_NAMES" json:"denied_mcp_names" yaml:"denied_mcp_names" mapstructure:"denied_mcp_names"`

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

	// ContextWarningThresholds defines usage ratios (0-1) that trigger telemetry warnings.
	ContextWarningThresholds []float64 `koanf:"context_warning_thresholds" env:"LLM_CONTEXT_WARNING_THRESHOLDS" json:"context_warning_thresholds" yaml:"context_warning_thresholds" mapstructure:"context_warning_thresholds"`
	// UsageMetrics configures observability knobs for usage persistence.
	//
	// Allows operators to tune histogram buckets collected by the usage repository.
	UsageMetrics LLMUsageMetricsConfig `koanf:"usage_metrics"                                                   json:"usage_metrics"              yaml:"usage_metrics"              mapstructure:"usage_metrics"`

	// DefaultTopP sets the default nucleus sampling threshold for all LLM requests.
	// Value of 0 means use the provider's default. Range: 0.0 to 1.0.
	// Default: 0.0
	DefaultTopP float64 `koanf:"default_top_p" json:"default_top_p" yaml:"default_top_p" mapstructure:"default_top_p" env:"LLM_DEFAULT_TOP_P"`

	// DefaultFrequencyPenalty sets the default penalty for token frequency.
	// Positive values reduce repetition. Range: -2.0 to 2.0.
	// Default: 0.0
	DefaultFrequencyPenalty float64 `koanf:"default_frequency_penalty" json:"default_frequency_penalty" yaml:"default_frequency_penalty" mapstructure:"default_frequency_penalty" env:"LLM_DEFAULT_FREQUENCY_PENALTY"`

	// DefaultPresencePenalty sets the default penalty for token presence.
	// Positive values encourage talking about new topics. Range: -2.0 to 2.0.
	// Default: 0.0
	DefaultPresencePenalty float64 `koanf:"default_presence_penalty" json:"default_presence_penalty" yaml:"default_presence_penalty" mapstructure:"default_presence_penalty" env:"LLM_DEFAULT_PRESENCE_PENALTY"`

	// DefaultSeed sets the default seed for reproducible outputs.
	// Value of 0 means non-deterministic (no seed).
	// Default: 0
	DefaultSeed int `koanf:"default_seed" json:"default_seed" yaml:"default_seed" mapstructure:"default_seed" env:"LLM_DEFAULT_SEED"`
}

// ToolCallCapsConfig captures default and per-tool invocation caps.
type ToolCallCapsConfig struct {
	Default   int            `koanf:"default"   json:"default"   yaml:"default"   mapstructure:"default"`
	Overrides map[string]int `koanf:"overrides" json:"overrides" yaml:"overrides" mapstructure:"overrides"`
}

// LLMUsageMetricsConfig exposes tuning knobs for usage repository telemetry.
//
// PersistBuckets defines histogram bucket boundaries (seconds) for persistence latency.
type LLMUsageMetricsConfig struct {
	PersistBuckets []float64 `koanf:"persist_buckets" json:"persist_buckets" yaml:"persist_buckets" mapstructure:"persist_buckets"`
}

// LLMRateLimitConfig defines shared throttling settings for provider calls.
//
// Disabled limiters allow unbounded concurrency, while enabling the limiter applies
// bounded worker pools with queueing to smooth spikes. Map overrides provide per-provider tuning.
type LLMRateLimitConfig struct {
	Enabled bool `koanf:"enabled" json:"enabled" yaml:"enabled" mapstructure:"enabled"`

	// DefaultConcurrency limits concurrent requests per provider when overrides are absent.
	// Zero defers to registry defaults; values beyond provider quotas are discouraged.
	DefaultConcurrency int `koanf:"default_concurrency" json:"default_concurrency" yaml:"default_concurrency" mapstructure:"default_concurrency" validate:"min=0"`

	// DefaultQueueSize bounds queued work waiting for a concurrency slot.
	// Zero disables queuing and causes immediate rejection when the pool is saturated.
	DefaultQueueSize int `koanf:"default_queue_size" json:"default_queue_size" yaml:"default_queue_size" mapstructure:"default_queue_size" validate:"min=0"`

	// DefaultRequestsPerMinute throttles average request throughput when per-provider overrides
	// are not supplied. Zero disables request-rate shaping.
	DefaultRequestsPerMinute int `koanf:"default_requests_per_minute" json:"default_requests_per_minute" yaml:"default_requests_per_minute" mapstructure:"default_requests_per_minute" validate:"min=0"`

	// DefaultTokensPerMinute constrains total tokens consumed per minute when overrides are absent.
	// Zero disables token-based shaping.
	DefaultTokensPerMinute int `koanf:"default_tokens_per_minute" json:"default_tokens_per_minute" yaml:"default_tokens_per_minute" mapstructure:"default_tokens_per_minute" validate:"min=0"`

	// DefaultRequestBurst overrides the burst size used for request-per-minute limiters.
	// Zero falls back to ceiling(perSecond) for compatibility.
	DefaultRequestBurst int `koanf:"default_request_burst" json:"default_request_burst" yaml:"default_request_burst" mapstructure:"default_request_burst" validate:"min=0"`

	// DefaultTokenBurst overrides the burst size used for token-per-minute limiters.
	// Zero falls back to ceiling(perSecond) for compatibility.
	DefaultTokenBurst int `koanf:"default_token_burst" json:"default_token_burst" yaml:"default_token_burst" mapstructure:"default_token_burst" validate:"min=0"`

	// DefaultReleaseSlotBeforeTokenWait releases concurrency slots before waiting on token budgets when true.
	// This favors throughput over strict slot ownership and may reduce head-of-line blocking.
	DefaultReleaseSlotBeforeTokenWait bool `koanf:"default_release_slot_before_token_wait" json:"default_release_slot_before_token_wait" yaml:"default_release_slot_before_token_wait" mapstructure:"default_release_slot_before_token_wait"`

	// PerProviderLimits customizes concurrency and queue depth for specific providers.
	// Keys should match provider names (e.g., "openai", "groq").
	PerProviderLimits map[string]ProviderRateLimitConfig `koanf:"per_provider_limits" json:"per_provider_limits" yaml:"per_provider_limits" mapstructure:"per_provider_limits"`
}

// ProviderRateLimitConfig describes concurrency limits for a single provider.
//
// Concurrency controls in-flight requests, while queue size bounds waiting work. Leaving
// fields at zero causes the limiter to fall back to global defaults.
type ProviderRateLimitConfig struct {
	Concurrency int `koanf:"concurrency"                    json:"concurrency"                    yaml:"concurrency"                    mapstructure:"concurrency"                    validate:"min=0"`
	QueueSize   int `koanf:"queue_size"                     json:"queue_size"                     yaml:"queue_size"                     mapstructure:"queue_size"                     validate:"min=0"`
	// RequestsPerMinute limits average throughput; zero disables the limiter.
	RequestsPerMinute int `koanf:"requests_per_minute"            json:"requests_per_minute"            yaml:"requests_per_minute"            mapstructure:"requests_per_minute"            validate:"min=0"`
	// TokensPerMinute constrains the total tokens consumed per minute; zero disables shaping.
	TokensPerMinute int `koanf:"tokens_per_minute"              json:"tokens_per_minute"              yaml:"tokens_per_minute"              mapstructure:"tokens_per_minute"              validate:"min=0"`
	// RequestBurst overrides the burst size for request-per-minute limiters. Zero defers to defaults.
	RequestBurst int `koanf:"request_burst"                  json:"request_burst"                  yaml:"request_burst"                  mapstructure:"request_burst"                  validate:"min=0"`
	// TokenBurst overrides the burst size for token-per-minute limiters. Zero defers to defaults.
	TokenBurst int `koanf:"token_burst"                    json:"token_burst"                    yaml:"token_burst"                    mapstructure:"token_burst"                    validate:"min=0"`
	// ReleaseSlotBeforeTokenWait releases concurrency slots before token waits when true; nil inherits defaults.
	ReleaseSlotBeforeTokenWait *bool `koanf:"release_slot_before_token_wait" json:"release_slot_before_token_wait" yaml:"release_slot_before_token_wait" mapstructure:"release_slot_before_token_wait"`
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
	// Mode controls Redis deployment model.
	//
	// Values:
	//   - "" (empty): Inherit from global Config.Mode
	//   - "memory": Use embedded Redis without persistence
	//   - "persistent": Use embedded Redis with persistence enabled
	//   - "distributed": Use external Redis (explicit override)
	Mode string `koanf:"mode" json:"mode" yaml:"mode" mapstructure:"mode" env:"REDIS_MODE" validate:"omitempty"`
	// URL provides a complete Redis connection string.
	//
	// Format: `redis://[user:password@]host:port/db`
	// Takes precedence over individual connection parameters.
	URL string `koanf:"url"  json:"url"  yaml:"url"  mapstructure:"url"  env:"REDIS_URL"`

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

	// Standalone config defines embedded Redis options used in memory and persistent modes.
	Standalone RedisStandaloneConfig `koanf:"standalone" json:"standalone" yaml:"standalone" mapstructure:"standalone"`
}

// RedisStandaloneConfig defines options for the embedded Redis used by memory and persistent modes.
type RedisStandaloneConfig struct {
	// Persistence configures optional snapshot persistence for embedded Redis.
	Persistence RedisPersistenceConfig `koanf:"persistence" json:"persistence" yaml:"persistence" mapstructure:"persistence"`
}

// RedisPersistenceConfig defines snapshot settings for embedded Redis.
type RedisPersistenceConfig struct {
	Enabled            bool          `koanf:"enabled"              json:"enabled"              yaml:"enabled"              mapstructure:"enabled"              env:"REDIS_STANDALONE_PERSISTENCE_ENABLED"`
	DataDir            string        `koanf:"data_dir"             json:"data_dir"             yaml:"data_dir"             mapstructure:"data_dir"             env:"REDIS_STANDALONE_PERSISTENCE_DATA_DIR"`
	SnapshotInterval   time.Duration `koanf:"snapshot_interval"    json:"snapshot_interval"    yaml:"snapshot_interval"    mapstructure:"snapshot_interval"    env:"REDIS_STANDALONE_PERSISTENCE_SNAPSHOT_INTERVAL"`
	SnapshotOnShutdown bool          `koanf:"snapshot_on_shutdown" json:"snapshot_on_shutdown" yaml:"snapshot_on_shutdown" mapstructure:"snapshot_on_shutdown" env:"REDIS_STANDALONE_PERSISTENCE_SNAPSHOT_ON_SHUTDOWN"`
	RestoreOnStartup   bool          `koanf:"restore_on_startup"   json:"restore_on_startup"   yaml:"restore_on_startup"   mapstructure:"restore_on_startup"   env:"REDIS_STANDALONE_PERSISTENCE_RESTORE_ON_STARTUP"`
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

	// KeyScanCount controls the COUNT hint used by Redis SCAN for key iteration.
	// Larger values reduce round-trips but may increase per-iteration latency.
	// Set to a positive integer; defaults to 100.
	KeyScanCount int `koanf:"key_scan_count" json:"key_scan_count" yaml:"key_scan_count" mapstructure:"key_scan_count" env:"CACHE_KEY_SCAN_COUNT" validate:"min=1"`
}

// AttachmentMIMEAllowlist holds allowed MIME types per category.
//
// Empty lists mean: use default per-type heuristics (not allow-all and not
// allow-none). Explicit values restrict to exact matches or "type/*" prefixes.
type AttachmentMIMEAllowlist struct {
	Image []string `koanf:"image" env:"ATTACHMENTS_ALLOWED_MIME_TYPES_IMAGE" json:"image" yaml:"image" mapstructure:"image"`
	Audio []string `koanf:"audio" env:"ATTACHMENTS_ALLOWED_MIME_TYPES_AUDIO" json:"audio" yaml:"audio" mapstructure:"audio"`
	Video []string `koanf:"video" env:"ATTACHMENTS_ALLOWED_MIME_TYPES_VIDEO" json:"video" yaml:"video" mapstructure:"video"`
	PDF   []string `koanf:"pdf"   env:"ATTACHMENTS_ALLOWED_MIME_TYPES_PDF"   json:"pdf"   yaml:"pdf"   mapstructure:"pdf"`
}

// AttachmentsConfig contains global limits and policies for attachment handling.
//
// These settings control how attachments are downloaded, validated, and processed
// across the system. They apply to all attachment resolutions regardless of scope.
type AttachmentsConfig struct {
	// MaxDownloadSizeBytes caps the maximum size (in bytes) for any single download.
	// Default: 10_000_000 (10MB)
	MaxDownloadSizeBytes int64 `koanf:"max_download_size_bytes" env:"ATTACHMENTS_MAX_DOWNLOAD_SIZE_BYTES" json:"max_download_size_bytes" yaml:"max_download_size_bytes" mapstructure:"max_download_size_bytes" validate:"min=1"`
	// DownloadTimeout sets the timeout for downloading a single attachment.
	// Default: 30s
	DownloadTimeout time.Duration `koanf:"download_timeout"        env:"ATTACHMENTS_DOWNLOAD_TIMEOUT"        json:"download_timeout"        yaml:"download_timeout"        mapstructure:"download_timeout"`
	// MaxRedirects limits the number of HTTP redirects followed during download.
	// Default: 3
	MaxRedirects int `koanf:"max_redirects"           env:"ATTACHMENTS_MAX_REDIRECTS"           json:"max_redirects"           yaml:"max_redirects"           mapstructure:"max_redirects"           validate:"min=0"`
	// AllowedMIMETypes specifies MIME allowlists by content category.
	//
	// Empty lists do not allow everything; they mean "use built-in defaults"
	// per attachment type. Resolvers fall back to type heuristics when the
	// allowlist for a category is empty.
	AllowedMIMETypes AttachmentMIMEAllowlist `koanf:"allowed_mime_types"                                                json:"allowed_mime_types"      yaml:"allowed_mime_types"      mapstructure:"allowed_mime_types"`
	// TempDirQuotaBytes optionally caps total temp storage used by attachment resolution.
	// 0 disables the quota.
	TempDirQuotaBytes int64 `koanf:"temp_dir_quota_bytes"    env:"ATTACHMENTS_TEMP_DIR_QUOTA_BYTES"    json:"temp_dir_quota_bytes"    yaml:"temp_dir_quota_bytes"    mapstructure:"temp_dir_quota_bytes"    validate:"min=0"`

	// TextPartMaxBytes caps the number of bytes loaded from text files into LLM parts.
	// Default: 5_242_880 (5MB)
	TextPartMaxBytes int64 `koanf:"text_part_max_bytes" env:"ATTACHMENTS_TEXT_PART_MAX_BYTES" json:"text_part_max_bytes" yaml:"text_part_max_bytes" mapstructure:"text_part_max_bytes" validate:"min=1"`

	// PDFExtractMaxChars caps the number of characters extracted from PDFs.
	// Default: 1_000_000
	PDFExtractMaxChars int `koanf:"pdf_extract_max_chars" env:"ATTACHMENTS_PDF_EXTRACT_MAX_CHARS" json:"pdf_extract_max_chars" yaml:"pdf_extract_max_chars" mapstructure:"pdf_extract_max_chars" validate:"min=1"`

	// HTTPUserAgent sets the User-Agent header for outbound downloads.
	// Default: "Compozy/1.0"
	HTTPUserAgent string `koanf:"http_user_agent" env:"ATTACHMENTS_HTTP_USER_AGENT" json:"http_user_agent" yaml:"http_user_agent" mapstructure:"http_user_agent"`

	// MIMEHeadMaxBytes controls how many initial bytes are used for MIME detection.
	// Default: 512
	MIMEHeadMaxBytes int `koanf:"mime_head_max_bytes" env:"ATTACHMENTS_MIME_HEAD_MAX_BYTES" json:"mime_head_max_bytes" yaml:"mime_head_max_bytes" mapstructure:"mime_head_max_bytes" validate:"min=1"`

	// SSRFStrict enforces blocking of local/loopback/private/multicast destinations even in tests.
	// Default: false
	SSRFStrict bool `koanf:"ssrf_strict" env:"ATTACHMENTS_SSRF_STRICT" json:"ssrf_strict" yaml:"ssrf_strict" mapstructure:"ssrf_strict"`
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

	// StartWorkflowTimeout bounds the HTTP handler's call to start a
	// workflow execution to avoid hanging requests when Temporal is slow
	// or unreachable. If zero or negative, a safe default is used.
	//
	// **Default**: `5s`
	StartWorkflowTimeout time.Duration `koanf:"start_workflow_timeout" json:"start_workflow_timeout" yaml:"start_workflow_timeout" mapstructure:"start_workflow_timeout" env:"WORKER_START_WORKFLOW_TIMEOUT"`

	// MaxConcurrentActivityExecutionSize bounds the number of activities a worker executes concurrently.
	//
	// **Default**: `0` (auto = 2x CPUs)
	MaxConcurrentActivityExecutionSize int `koanf:"max_concurrent_activity_execution_size" json:"max_concurrent_activity_execution_size" yaml:"max_concurrent_activity_execution_size" mapstructure:"max_concurrent_activity_execution_size" env:"WORKER_MAX_CONCURRENT_ACTIVITIES"`

	// MaxConcurrentWorkflowExecutionSize bounds the number of workflow tasks executed concurrently.
	//
	// **Default**: `0` (auto = 1x CPUs)
	MaxConcurrentWorkflowExecutionSize int `koanf:"max_concurrent_workflow_execution_size" json:"max_concurrent_workflow_execution_size" yaml:"max_concurrent_workflow_execution_size" mapstructure:"max_concurrent_workflow_execution_size" env:"WORKER_MAX_CONCURRENT_WORKFLOWS"`

	// MaxConcurrentLocalActivityExecutionSize bounds concurrently executing local activities.
	//
	// **Default**: `0` (auto = 4x CPUs)
	MaxConcurrentLocalActivityExecutionSize int `koanf:"max_concurrent_local_activity_execution_size" json:"max_concurrent_local_activity_execution_size" yaml:"max_concurrent_local_activity_execution_size" mapstructure:"max_concurrent_local_activity_execution_size" env:"WORKER_MAX_CONCURRENT_LOCAL_ACTIVITIES"`

	// ActivityStartToCloseTimeout defines the default bounded execution time for retryable activities.
	//
	// **Default**: `5m`
	ActivityStartToCloseTimeout time.Duration `koanf:"activity_start_to_close_timeout" json:"activity_start_to_close_timeout" yaml:"activity_start_to_close_timeout" mapstructure:"activity_start_to_close_timeout" env:"WORKER_ACTIVITY_START_TO_CLOSE_TIMEOUT"`

	// ActivityHeartbeatTimeout defines the default heartbeat window for long-running activities.
	//
	// **Default**: `30s`
	ActivityHeartbeatTimeout time.Duration `koanf:"activity_heartbeat_timeout" json:"activity_heartbeat_timeout" yaml:"activity_heartbeat_timeout" mapstructure:"activity_heartbeat_timeout" env:"WORKER_ACTIVITY_HEARTBEAT_TIMEOUT"`

	// ActivityMaxRetries bounds retry attempts for default activity execution.
	//
	// **Default**: `3`
	ActivityMaxRetries int `koanf:"activity_max_retries" json:"activity_max_retries" yaml:"activity_max_retries" mapstructure:"activity_max_retries" env:"WORKER_ACTIVITY_MAX_RETRIES"`

	// ErrorHandlerTimeout bounds retries invoked during workflow failure handling logic.
	//
	// **Default**: `30s`
	ErrorHandlerTimeout time.Duration `koanf:"error_handler_timeout" json:"error_handler_timeout" yaml:"error_handler_timeout" mapstructure:"error_handler_timeout" env:"WORKER_ERROR_HANDLER_TIMEOUT"`

	// ErrorHandlerMaxRetries caps retry attempts for error handling activities.
	//
	// **Default**: `3`
	ErrorHandlerMaxRetries int `koanf:"error_handler_max_retries" json:"error_handler_max_retries" yaml:"error_handler_max_retries" mapstructure:"error_handler_max_retries" env:"WORKER_ERROR_HANDLER_MAX_RETRIES"`

	// Dispatcher defines heartbeat tracking for dispatcher leases.
	Dispatcher WorkerDispatcherConfig `koanf:"dispatcher" json:"dispatcher" yaml:"dispatcher" mapstructure:"dispatcher"`
}

// WorkerDispatcherConfig holds dispatcher heartbeat tracking configuration.
type WorkerDispatcherConfig struct {
	HeartbeatTTL   time.Duration `koanf:"heartbeat_ttl"   json:"heartbeat_ttl"   yaml:"heartbeat_ttl"   mapstructure:"heartbeat_ttl"   env:"WORKER_DISPATCHER_HEARTBEAT_TTL"`
	StaleThreshold time.Duration `koanf:"stale_threshold" json:"stale_threshold" yaml:"stale_threshold" mapstructure:"stale_threshold" env:"WORKER_DISPATCHER_STALE_THRESHOLD"`
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
//	  port: 0           # 0 = ephemeral; actual port is logged
//	  base_url: ""      # auto-computed from bound address when empty
type MCPProxyConfig struct {
	// Mode controls how the MCP proxy runs within Compozy.
	//
	// Values:
	//   - "memory": Embed the MCP proxy inside the server with in-memory state
	//   - "persistent": Embed the MCP proxy with durable on-disk state
	//   - "distributed": Delegate to an external MCP proxy endpoint
	//   - "": Inherit the global deployment mode (default)
	//
	// When embedded, the server manages lifecycle and health of the proxy
	// and will set LLM.ProxyURL if empty.
	Mode string `koanf:"mode" json:"mode" yaml:"mode" mapstructure:"mode" env:"MCP_PROXY_MODE"`
	// Host specifies the network interface to bind the MCP proxy server to.
	//
	// **Default**: `"0.0.0.0"`
	Host string `koanf:"host" json:"host" yaml:"host" mapstructure:"host" env:"MCP_PROXY_HOST"`

	// Port specifies the TCP port for the MCP proxy server.
	//
	// **Default**: `0` (ephemeral)
	Port int `koanf:"port" json:"port" yaml:"port" mapstructure:"port" env:"MCP_PROXY_PORT"`

	// BaseURL specifies the base URL for MCP proxy API endpoints.
	//
	// **Default**: `""` (empty string)
	BaseURL string `koanf:"base_url" json:"base_url" yaml:"base_url" mapstructure:"base_url" env:"MCP_PROXY_BASE_URL"`

	// ShutdownTimeout sets timeout for graceful shutdown.
	//
	// **Default**: `30s`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"        json:"shutdown_timeout"        yaml:"shutdown_timeout"        mapstructure:"shutdown_timeout"        env:"MCP_PROXY_SHUTDOWN_TIMEOUT"`
	// MaxIdleConns controls the maximum number of idle (keep-alive) connections across all hosts.
	//
	// **Default**: `128`
	MaxIdleConns int `koanf:"max_idle_conns"          json:"max_idle_conns"          yaml:"max_idle_conns"          mapstructure:"max_idle_conns"          env:"MCP_PROXY_MAX_IDLE_CONNS"`
	// MaxIdleConnsPerHost controls the maximum idle (keep-alive) connections to keep per-host.
	//
	// **Default**: `128`
	MaxIdleConnsPerHost int `koanf:"max_idle_conns_per_host" json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host" mapstructure:"max_idle_conns_per_host" env:"MCP_PROXY_MAX_IDLE_CONNS_PER_HOST"`
	// MaxConnsPerHost caps the total number of simultaneous connections per host.
	//
	// **Default**: `128`
	MaxConnsPerHost int `koanf:"max_conns_per_host"      json:"max_conns_per_host"      yaml:"max_conns_per_host"      mapstructure:"max_conns_per_host"      env:"MCP_PROXY_MAX_CONNS_PER_HOST"`
	// IdleConnTimeout is the maximum amount of time an idle (keep-alive) connection will remain
	// idle before closing itself.
	//
	// **Default**: `90s`
	IdleConnTimeout time.Duration `koanf:"idle_conn_timeout"       json:"idle_conn_timeout"       yaml:"idle_conn_timeout"       mapstructure:"idle_conn_timeout"       env:"MCP_PROXY_IDLE_CONN_TIMEOUT"`
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
const (
	DefaultPortReleaseTimeout        = 5 * time.Second
	DefaultPortReleasePollInterval   = 100 * time.Millisecond
	DefaultCLIMaxRetries             = 3
	DefaultCLIActiveWindowDays       = 30
	DefaultCLIDevWatcherDebounce     = 200 * time.Millisecond
	DefaultCLIDevWatcherInitialDelay = 500 * time.Millisecond
	DefaultCLIDevWatcherMaxDelay     = 30 * time.Second
)

// CLIDevConfig contains development tooling settings for the CLI.
//
// These options let operators tune hot-reload behavior when running
// `compozy dev`, including how aggressively file changes trigger restarts.
type CLIDevConfig struct {
	// WatcherDebounce defines the quiet period before restarting the dev server after a file change.
	// Lower values trigger faster restarts; higher values reduce churn when many files change at once.
	//
	// **Default**: `200ms`
	WatcherDebounce time.Duration `koanf:"watcher_debounce" env:"COMPOZY_DEV_WATCHER_DEBOUNCE" json:"WatcherDebounce" yaml:"watcher_debounce" mapstructure:"watcher_debounce" validate:"min=0"`

	// WatcherRetryInitial controls the first backoff duration after an unexpected server failure.
	// The delay doubles after each failure until WatcherRetryMax is reached.
	//
	// **Default**: `500ms`
	WatcherRetryInitial time.Duration `koanf:"watcher_retry_initial" env:"COMPOZY_DEV_WATCHER_RETRY_INITIAL" json:"WatcherRetryInitial" yaml:"watcher_retry_initial" mapstructure:"watcher_retry_initial" validate:"min=0"`

	// WatcherRetryMax caps the exponential backoff window when the dev server repeatedly fails to start.
	//
	// **Default**: `30s`
	WatcherRetryMax time.Duration `koanf:"watcher_retry_max" env:"COMPOZY_DEV_WATCHER_RETRY_MAX" json:"WatcherRetryMax" yaml:"watcher_retry_max" mapstructure:"watcher_retry_max" validate:"min=0"`
}

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
	Mode string `koanf:"mode" env:"COMPOZY_CLI_MODE" json:"Mode" yaml:"mode" mapstructure:"mode"`

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

	// PortReleaseTimeout sets the maximum time to wait for a port to become available.
	//
	// Default: 5s
	PortReleaseTimeout time.Duration `koanf:"port_release_timeout" env:"COMPOZY_PORT_RELEASE_TIMEOUT" json:"PortReleaseTimeout" yaml:"port_release_timeout" mapstructure:"port_release_timeout" validate:"min=0"`

	// PortReleasePollInterval sets how often to check if a port has become available.
	//
	// Default: 100ms
	PortReleasePollInterval time.Duration `koanf:"port_release_poll_interval" env:"COMPOZY_PORT_RELEASE_POLL_INTERVAL" json:"PortReleasePollInterval" yaml:"port_release_poll_interval" mapstructure:"port_release_poll_interval" validate:"min=0"`

	// FileWatchInterval controls the polling cadence when filesystem notifications are unavailable.
	//
	// Default: 1s
	// Set to 0 to use the built-in default.
	FileWatchInterval time.Duration `koanf:"file_watch_interval" env:"COMPOZY_FILE_WATCH_INTERVAL" json:"FileWatchInterval" yaml:"file_watch_interval" mapstructure:"file_watch_interval" validate:"min=0"`

	// MaxRetries sets the maximum retry attempts for CLI HTTP requests.
	// Default: 3. Set to a non-negative value; 0 reverts to the default and negative disables retries.
	MaxRetries int `koanf:"max_retries" env:"COMPOZY_MAX_RETRIES" json:"MaxRetries" yaml:"max_retries" mapstructure:"max_retries"`

	// Dev exposes local development settings, including watcher debounce and restart backoff.
	Dev CLIDevConfig `koanf:"dev" json:"Dev" yaml:"dev" mapstructure:"dev"`

	// Users configures CLI behavior for user-management commands.
	//
	// Provides operator-tunable knobs for filters and heuristics like the active-user window.
	Users CLIUsersConfig `koanf:"users" json:"Users" yaml:"users" mapstructure:"users"`
}

// CLIUsersConfig controls CLI user-management heuristics and filters.
type CLIUsersConfig struct {
	// ActiveWindowDays specifies how many days define an "active" user.
	//
	// Used by commands like `auth users list --active` to determine recent activity.
	// Default: 30 days.
	ActiveWindowDays int `koanf:"active_window_days" env:"COMPOZY_USERS_ACTIVE_WINDOW_DAYS" json:"ActiveWindowDays" yaml:"active_window_days" mapstructure:"active_window_days" validate:"min=0"`
}

// WebhooksConfig contains webhook processing and validation configuration.
// These settings control how incoming webhooks are processed, validated,
// and deduplicated across the system.
//
// Example configuration:
//
//	webhooks:
//	  default_method: POST
//	  default_max_body: 1048576        # 1MB
//	  default_dedupe_ttl: 10m          # 10 minutes
//	  stripe_skew: 5m                  # 5 minutes
type WebhooksConfig struct {
	// DefaultMethod specifies the default HTTP method for webhook requests.
	// Default: "POST"
	// Valid values: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
	DefaultMethod string `koanf:"default_method" json:"default_method" yaml:"default_method" mapstructure:"default_method" env:"WEBHOOKS_DEFAULT_METHOD" validate:"oneof=GET POST PUT DELETE PATCH HEAD OPTIONS"`

	// DefaultMaxBody caps the maximum size (in bytes) for webhook request bodies.
	// Default: 1,048,576 (1MB)
	// Must be greater than 0
	DefaultMaxBody int64 `koanf:"default_max_body" json:"default_max_body" yaml:"default_max_body" mapstructure:"default_max_body" env:"WEBHOOKS_DEFAULT_MAX_BODY" validate:"min=1"`

	// DefaultDedupeTTL sets the default time-to-live for webhook deduplication.
	// Default: 10m (10 minutes)
	// Must be non-negative
	DefaultDedupeTTL time.Duration `koanf:"default_dedupe_ttl" json:"default_dedupe_ttl" yaml:"default_dedupe_ttl" mapstructure:"default_dedupe_ttl" env:"WEBHOOKS_DEFAULT_DEDUPE_TTL" validate:"min=0"`

	// StripeSkew sets the allowed timestamp skew for Stripe webhook verification.
	// Default: 5m (5 minutes)
	// Must be non-negative
	StripeSkew time.Duration `koanf:"stripe_skew" json:"stripe_skew" yaml:"stripe_skew" mapstructure:"stripe_skew" env:"WEBHOOKS_STRIPE_SKEW" validate:"min=0"`
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
		Mode:        getString(registry, "mode"),
		Server:      buildServerConfig(registry),
		Database:    buildDatabaseConfig(registry),
		Temporal:    buildTemporalConfig(registry),
		Runtime:     buildRuntimeConfig(registry),
		Stream:      buildStreamConfig(registry),
		Tasks:       buildTasksConfig(registry),
		Limits:      buildLimitsConfig(registry),
		Attachments: buildAttachmentsConfig(registry),
		Memory:      buildMemoryConfig(registry),
		Knowledge:   buildKnowledgeConfig(registry),
		LLM:         buildLLMConfig(registry),
		RateLimit:   buildRateLimitConfig(registry),
		CLI:         buildCLIConfig(registry),
		Redis:       buildRedisConfig(registry),
		Cache:       buildCacheConfig(registry),
		Worker:      buildWorkerConfig(registry),
		MCPProxy:    buildMCPProxyConfig(registry),
		Webhooks:    buildWebhooksConfig(registry),
	}
}

func buildStreamConfig(registry *definition.Registry) StreamConfig {
	return StreamConfig{
		Agent: AgentStreamConfig{
			DefaultPoll:        getDuration(registry, "stream.agent.default_poll"),
			MinPoll:            getDuration(registry, "stream.agent.min_poll"),
			MaxPoll:            getDuration(registry, "stream.agent.max_poll"),
			HeartbeatFrequency: getDuration(registry, "stream.agent.heartbeat_frequency"),
			ReplayLimit:        getInt(registry, "stream.agent.replay_limit"),
		},
		Task: TaskStreamEndpointConfig{
			DefaultPoll:        getDuration(registry, "stream.task.default_poll"),
			MinPoll:            getDuration(registry, "stream.task.min_poll"),
			MaxPoll:            getDuration(registry, "stream.task.max_poll"),
			HeartbeatFrequency: getDuration(registry, "stream.task.heartbeat_frequency"),
			RedisChannelPrefix: getString(registry, "stream.task.redis_channel_prefix"),
			RedisLogPrefix:     getString(registry, "stream.task.redis_log_prefix"),
			RedisSeqPrefix:     getString(registry, "stream.task.redis_seq_prefix"),
			RedisMaxEntries:    getInt64(registry, "stream.task.redis_max_entries"),
			RedisTTL:           getDuration(registry, "stream.task.redis_ttl"),
			ReplayLimit:        getInt(registry, "stream.task.replay_limit"),
			Text: TaskTextStreamConfig{
				MaxSegmentRunes: getInt(registry, "stream.task.text.max_segment_runes"),
				PublishTimeout:  getDuration(registry, "stream.task.text.publish_timeout"),
			},
		},
		LLM: LLMStreamConfig{
			FallbackSegmentLimit: getInt(registry, "stream.llm.fallback_segment_limit"),
		},
		Workflow: WorkflowStreamConfig{
			DefaultPoll:        getDuration(registry, "stream.workflow.default_poll"),
			MinPoll:            getDuration(registry, "stream.workflow.min_poll"),
			MaxPoll:            getDuration(registry, "stream.workflow.max_poll"),
			HeartbeatFrequency: getDuration(registry, "stream.workflow.heartbeat_frequency"),
			QueryTimeout:       getDuration(registry, "stream.workflow.query_timeout"),
		},
	}
}

func buildTasksConfig(registry *definition.Registry) TasksConfig {
	return TasksConfig{
		Retry: TaskRetryConfig{
			ChildState: TaskChildStateRetryConfig{
				MaxAttempts: getInt(registry, "tasks.retry.child_state.max_attempts"),
				BaseBackoff: getDuration(registry, "tasks.retry.child_state.base_backoff"),
			},
		},
		Wait: TaskWaitConfig{
			Siblings: TaskSiblingWaitConfig{
				PollInterval: getDuration(registry, "tasks.wait.siblings.poll_interval"),
				Timeout:      getDuration(registry, "tasks.wait.siblings.timeout"),
			},
		},
		Stream: TaskStreamConfig{
			MaxChunks: getInt(registry, "tasks.stream.max_chunks"),
		},
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

func getFloat64(registry *definition.Registry, path string) float64 {
	if val := registry.GetDefault(path); val != nil {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return 0
}

func getFloat64Slice(registry *definition.Registry, path string) []float64 {
	val := registry.GetDefault(path)
	if val == nil {
		return nil
	}
	switch raw := val.(type) {
	case []float64:
		if len(raw) == 0 {
			return nil
		}
		out := make([]float64, len(raw))
		copy(out, raw)
		return out
	case []any:
		out := make([]float64, 0, len(raw))
		for _, item := range raw {
			switch num := item.(type) {
			case float64:
				out = append(out, num)
			case float32:
				out = append(out, float64(num))
			case int:
				out = append(out, float64(num))
			case int64:
				out = append(out, float64(num))
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

func getStringSlice(registry *definition.Registry, path string) []string {
	if val := registry.GetDefault(path); val != nil {
		if slice, ok := val.([]string); ok {
			return slice
		}
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

func getStringIntMap(registry *definition.Registry, path string) map[string]int {
	val := registry.GetDefault(path)
	if val == nil {
		return nil
	}
	out := make(map[string]int)
	switch raw := val.(type) {
	case map[string]int:
		for k, v := range raw {
			out[k] = v
		}
	case map[string]any:
		for k, v := range raw {
			switch num := v.(type) {
			case int:
				out[k] = num
			case int64:
				out[k] = int(num)
			case float64:
				out[k] = int(num)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func buildNativeExecAllowlist(registry *definition.Registry) []NativeExecCommandConfig {
	raw := getMapSlice(registry, "runtime.native_tools.exec.allowlist")
	if len(raw) == 0 {
		return nil
	}
	result := make([]NativeExecCommandConfig, 0, len(raw))
	for _, item := range raw {
		if item == nil {
			continue
		}
		var cfg NativeExecCommandConfig
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName:          "mapstructure",
			Result:           &cfg,
			WeaklyTypedInput: true,
			DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		})
		if err != nil {
			continue
		}
		if err := decoder.Decode(item); err != nil {
			continue
		}
		result = append(result, cfg)
	}
	return result
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
			Enabled:                      getBool(registry, "server.auth.enabled"),
			WorkflowExceptions:           getStringSlice(registry, "server.auth.workflow_exceptions"),
			AdminKey:                     SensitiveString(getString(registry, "server.auth.admin_key")),
			APIKeyLastUsedMaxConcurrency: getInt(registry, "server.auth.api_key_last_used_max_concurrency"),
			APIKeyLastUsedTimeout:        getDuration(registry, "server.auth.api_key_last_used_timeout"),
		},
		SourceOfTruth:       getString(registry, "server.source_of_truth"),
		SeedFromRepoOnEmpty: getBool(registry, "server.seed_from_repo_on_empty"),
		Timeouts:            buildServerTimeouts(registry),
		Reconciler:          buildReconcilerConfig(registry),
	}
}

func buildServerTimeouts(registry *definition.Registry) ServerTimeouts {
	return ServerTimeouts{
		MonitoringInit:              getDuration(registry, "server.timeouts.monitoring_init"),
		MonitoringShutdown:          getDuration(registry, "server.timeouts.monitoring_shutdown"),
		DBShutdown:                  getDuration(registry, "server.timeouts.db_shutdown"),
		WorkerShutdown:              getDuration(registry, "server.timeouts.worker_shutdown"),
		ServerShutdown:              getDuration(registry, "server.timeouts.server_shutdown"),
		HTTPRead:                    getDuration(registry, "server.timeouts.http_read"),
		HTTPWrite:                   getDuration(registry, "server.timeouts.http_write"),
		HTTPIdle:                    getDuration(registry, "server.timeouts.http_idle"),
		ScheduleRetryMaxDuration:    getDuration(registry, "server.timeouts.schedule_retry_max_duration"),
		ScheduleRetryBaseDelay:      getDuration(registry, "server.timeouts.schedule_retry_base_delay"),
		ScheduleRetryMaxDelay:       getDuration(registry, "server.timeouts.schedule_retry_max_delay"),
		ScheduleRetryMaxAttempts:    getInt(registry, "server.timeouts.schedule_retry_max_attempts"),
		ScheduleRetryBackoffSeconds: getInt(registry, "server.timeouts.schedule_retry_backoff_seconds"),
		KnowledgeIngest:             getDuration(registry, "server.timeouts.knowledge_ingest"),
		TemporalReachability:        getDuration(registry, "server.timeouts.temporal_reachability"),
		StartProbeDelay:             getDuration(registry, "server.timeouts.start_probe_delay"),
	}
}

const (
	DefaultReconcilerQueueCapacity   = 1024
	DefaultReconcilerDebounceWait    = 300 * time.Millisecond
	DefaultReconcilerDebounceMaxWait = 500 * time.Millisecond
)

func buildReconcilerConfig(registry *definition.Registry) ReconcilerConfig {
	cfg := ReconcilerConfig{
		QueueCapacity:   getInt(registry, "server.reconciler.queue_capacity"),
		DebounceWait:    getDuration(registry, "server.reconciler.debounce_wait"),
		DebounceMaxWait: getDuration(registry, "server.reconciler.debounce_max_wait"),
	}
	if cfg.QueueCapacity <= 0 {
		cfg.QueueCapacity = DefaultReconcilerQueueCapacity
	}
	if cfg.DebounceWait <= 0 {
		cfg.DebounceWait = DefaultReconcilerDebounceWait
	}
	if cfg.DebounceMaxWait <= 0 {
		cfg.DebounceMaxWait = DefaultReconcilerDebounceMaxWait
	}
	return cfg
}

func buildDatabaseConfig(registry *definition.Registry) DatabaseConfig {
	return DatabaseConfig{
		ConnString:       getString(registry, "database.conn_string"),
		Driver:           getString(registry, "database.driver"),
		Host:             getString(registry, "database.host"),
		Port:             getString(registry, "database.port"),
		User:             getString(registry, "database.user"),
		Password:         getString(registry, "database.password"),
		DBName:           getString(registry, "database.name"),
		SSLMode:          getString(registry, "database.ssl_mode"),
		AutoMigrate:      getBool(registry, "database.auto_migrate"),
		MigrationTimeout: getDuration(registry, "database.migration_timeout"),
		MaxOpenConns:     getInt(registry, "database.max_open_conns"),
		MaxIdleConns:     getInt(registry, "database.max_idle_conns"),
		ConnMaxLifetime:  getDuration(registry, "database.conn_max_lifetime"),
		ConnMaxIdleTime:  getDuration(registry, "database.conn_max_idle_time"),
		PingTimeout:      getDuration(registry, "database.ping_timeout"),
		HealthCheckTimeout: getDuration(
			registry,
			"database.health_check_timeout",
		),
		HealthCheckPeriod: getDuration(
			registry,
			"database.health_check_period",
		),
		ConnectTimeout: getDuration(registry, "database.connect_timeout"),
		Path:           getString(registry, "database.path"),
		BusyTimeout:    getDuration(registry, "database.busy_timeout"),
	}
}

func buildTemporalConfig(registry *definition.Registry) TemporalConfig {
	return TemporalConfig{
		Mode:      getString(registry, "temporal.mode"),
		HostPort:  getString(registry, "temporal.host_port"),
		Namespace: getString(registry, "temporal.namespace"),
		TaskQueue: getString(registry, "temporal.task_queue"),
		Standalone: StandaloneConfig{
			DatabaseFile: getString(registry, "temporal.standalone.database_file"),
			FrontendPort: getInt(registry, "temporal.standalone.frontend_port"),
			BindIP:       getString(registry, "temporal.standalone.bind_ip"),
			Namespace:    getString(registry, "temporal.standalone.namespace"),
			ClusterName:  getString(registry, "temporal.standalone.cluster_name"),
			EnableUI:     getBool(registry, "temporal.standalone.enable_ui"),
			RequireUI:    getBool(registry, "temporal.standalone.require_ui"),
			UIPort:       getInt(registry, "temporal.standalone.ui_port"),
			LogLevel:     getString(registry, "temporal.standalone.log_level"),
			StartTimeout: getDuration(registry, "temporal.standalone.start_timeout"),
		},
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
		TaskExecutionTimeoutDefault: getDuration(registry, "runtime.task_execution_timeout_default"),
		TaskExecutionTimeoutMax:     getDuration(registry, "runtime.task_execution_timeout_max"),
		ToolExecutionTimeout:        getDuration(registry, "runtime.tool_execution_timeout"),
		RuntimeType:                 getString(registry, "runtime.runtime_type"),
		EntrypointPath:              getString(registry, "runtime.entrypoint_path"),
		BunPermissions:              getStringSlice(registry, "runtime.bun_permissions"),
		NativeTools:                 buildNativeToolsConfig(registry),
	}
}

func buildNativeToolsConfig(registry *definition.Registry) NativeToolsConfig {
	return NativeToolsConfig{
		Enabled:         getBool(registry, "runtime.native_tools.enabled"),
		RootDir:         getString(registry, "runtime.native_tools.root_dir"),
		AdditionalRoots: getStringSlice(registry, "runtime.native_tools.additional_roots"),
		Exec:            buildNativeExecConfig(registry),
		Fetch:           buildNativeFetchConfig(registry),
		CallAgent:       buildNativeCallAgentConfig(registry),
		CallAgents:      buildNativeCallAgentsConfig(registry),
		CallTask:        buildNativeCallTaskConfig(registry),
		CallTasks:       buildNativeCallTasksConfig(registry),
		CallWorkflow:    buildNativeCallWorkflowConfig(registry),
		CallWorkflows:   buildNativeCallWorkflowsConfig(registry),
	}
}

func buildNativeExecConfig(registry *definition.Registry) NativeExecConfig {
	return NativeExecConfig{
		Timeout:        getDuration(registry, "runtime.native_tools.exec.timeout"),
		MaxStdoutBytes: getInt64(registry, "runtime.native_tools.exec.max_stdout_bytes"),
		MaxStderrBytes: getInt64(registry, "runtime.native_tools.exec.max_stderr_bytes"),
		Allowlist:      buildNativeExecAllowlist(registry),
	}
}

func buildNativeFetchConfig(registry *definition.Registry) NativeFetchConfig {
	return NativeFetchConfig{
		Timeout:        getDuration(registry, "runtime.native_tools.fetch.timeout"),
		MaxBodyBytes:   getInt64(registry, "runtime.native_tools.fetch.max_body_bytes"),
		MaxRedirects:   getInt(registry, "runtime.native_tools.fetch.max_redirects"),
		AllowedMethods: getStringSlice(registry, "runtime.native_tools.fetch.allowed_methods"),
	}
}

func buildNativeCallAgentConfig(registry *definition.Registry) NativeCallAgentConfig {
	return NativeCallAgentConfig{
		Enabled:        getBool(registry, "runtime.native_tools.call_agent.enabled"),
		DefaultTimeout: getDuration(registry, "runtime.native_tools.call_agent.default_timeout"),
	}
}

func buildNativeCallAgentsConfig(registry *definition.Registry) NativeCallAgentsConfig {
	return NativeCallAgentsConfig{
		Enabled:        getBool(registry, "runtime.native_tools.call_agents.enabled"),
		DefaultTimeout: getDuration(registry, "runtime.native_tools.call_agents.default_timeout"),
		MaxConcurrent:  getInt(registry, "runtime.native_tools.call_agents.max_concurrent"),
	}
}

func buildNativeCallTaskConfig(registry *definition.Registry) NativeCallTaskConfig {
	return NativeCallTaskConfig{
		Enabled:        getBool(registry, "runtime.native_tools.call_task.enabled"),
		DefaultTimeout: getDuration(registry, "runtime.native_tools.call_task.default_timeout"),
	}
}

func buildNativeCallTasksConfig(registry *definition.Registry) NativeCallTasksConfig {
	return NativeCallTasksConfig{
		Enabled:        getBool(registry, "runtime.native_tools.call_tasks.enabled"),
		DefaultTimeout: getDuration(registry, "runtime.native_tools.call_tasks.default_timeout"),
		MaxConcurrent:  getInt(registry, "runtime.native_tools.call_tasks.max_concurrent"),
	}
}

func buildNativeCallWorkflowConfig(registry *definition.Registry) NativeCallWorkflowConfig {
	return NativeCallWorkflowConfig{
		Enabled:        getBool(registry, "runtime.native_tools.call_workflow.enabled"),
		DefaultTimeout: getDuration(registry, "runtime.native_tools.call_workflow.default_timeout"),
	}
}

func buildNativeCallWorkflowsConfig(registry *definition.Registry) NativeCallWorkflowsConfig {
	return NativeCallWorkflowsConfig{
		Enabled:        getBool(registry, "runtime.native_tools.call_workflows.enabled"),
		DefaultTimeout: getDuration(registry, "runtime.native_tools.call_workflows.default_timeout"),
		MaxConcurrent:  getInt(registry, "runtime.native_tools.call_workflows.max_concurrent"),
	}
}

func buildLimitsConfig(registry *definition.Registry) LimitsConfig {
	return LimitsConfig{
		MaxNestingDepth:           getInt(registry, "limits.max_nesting_depth"),
		MaxConfigFileNestingDepth: getInt(registry, "limits.max_config_file_nesting_depth"),
		MaxStringLength:           getInt(registry, "limits.max_string_length"),
		MaxConfigFileSize:         getInt(registry, "limits.max_config_file_size"),
		MaxMessageContent:         getInt(registry, "limits.max_message_content"),
		MaxTotalContentSize:       getInt(registry, "limits.max_total_content_size"),
		MaxTaskContextDepth:       getInt(registry, "limits.max_task_context_depth"),
		ParentUpdateBatchSize:     getInt(registry, "limits.parent_update_batch_size"),
	}
}

func buildMemoryConfig(registry *definition.Registry) MemoryConfig {
	return MemoryConfig{
		Prefix:     getString(registry, "memory.prefix"),
		TTL:        getDuration(registry, "memory.ttl"),
		MaxEntries: getInt(registry, "memory.max_entries"),
	}
}

func buildKnowledgeConfig(registry *definition.Registry) KnowledgeConfig {
	return KnowledgeConfig{
		EmbedderBatchSize:        getInt(registry, "knowledge.embedder_batch_size"),
		ChunkSize:                getInt(registry, "knowledge.chunk_size"),
		ChunkOverlap:             getInt(registry, "knowledge.chunk_overlap"),
		RetrievalTopK:            getInt(registry, "knowledge.retrieval_top_k"),
		RetrievalMinScore:        getFloat64(registry, "knowledge.retrieval_min_score"),
		MaxMarkdownFileSizeBytes: getInt(registry, "knowledge.max_markdown_file_size_bytes"),
		VectorHTTPTimeout:        getDuration(registry, "knowledge.vector_http_timeout"),
		VectorDBs:                nil,
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
		ProviderTimeout:            getDuration(registry, "llm.provider_timeout"),
		RetryJitter:                getBool(registry, "llm.retry_jitter"),
		MaxConcurrentTools:         getInt(registry, "llm.max_concurrent_tools"),
		MaxToolIterations:          getInt(registry, "llm.max_tool_iterations"),
		MaxSequentialToolErrors:    getInt(registry, "llm.max_sequential_tool_errors"),
		MaxConsecutiveSuccesses:    getInt(registry, "llm.max_consecutive_successes"),
		EnableProgressTracking:     getBool(registry, "llm.enable_progress_tracking"),
		NoProgressThreshold:        getInt(registry, "llm.no_progress_threshold"),
		EnableLoopRestarts:         getBool(registry, "llm.enable_loop_restarts"),
		RestartStallThreshold:      getInt(registry, "llm.restart_stall_threshold"),
		MaxLoopRestarts:            getInt(registry, "llm.max_loop_restarts"),
		EnableContextCompaction:    getBool(registry, "llm.enable_context_compaction"),
		ContextCompactionThreshold: getFloat64(registry, "llm.context_compaction_threshold"),
		ContextCompactionCooldown:  getInt(registry, "llm.context_compaction_cooldown"),
		EnableDynamicPromptState:   getBool(registry, "llm.enable_dynamic_prompt_state"),
		ToolCallCaps: ToolCallCapsConfig{
			Default:   getInt(registry, "llm.tool_call_caps.default"),
			Overrides: getStringIntMap(registry, "llm.tool_call_caps.overrides"),
		},
		StructuredOutputRetryAttempts: getInt(registry, "llm.structured_output_retries"),
		FinalizeOutputRetryAttempts:   getInt(registry, "llm.finalize_output_retries"),
		AllowedMCPNames:               getStringSlice(registry, "llm.allowed_mcp_names"),
		DeniedMCPNames:                getStringSlice(registry, "llm.denied_mcp_names"),
		FailOnMCPRegistrationError:    getBool(registry, "llm.fail_on_mcp_registration_error"),
		RegisterMCPs:                  getMapSlice(registry, "llm.register_mcps"),
		MCPClientTimeout:              getDuration(registry, "llm.mcp_client_timeout"),
		RetryJitterPercent:            getInt(registry, "llm.retry_jitter_percent"),
		ContextWarningThresholds:      getFloat64Slice(registry, "llm.context_warning_thresholds"),
		UsageMetrics: LLMUsageMetricsConfig{
			PersistBuckets: getFloat64Slice(registry, "llm.usage_metrics.persist_buckets"),
		},
		RateLimiting:            buildLLMRateLimitConfig(registry),
		DefaultTopP:             getFloat64(registry, "llm.default_top_p"),
		DefaultFrequencyPenalty: getFloat64(registry, "llm.default_frequency_penalty"),
		DefaultPresencePenalty:  getFloat64(registry, "llm.default_presence_penalty"),
		DefaultSeed:             getInt(registry, "llm.default_seed"),
	}
}

func buildLLMRateLimitConfig(registry *definition.Registry) LLMRateLimitConfig {
	cfg := LLMRateLimitConfig{
		Enabled:                  getBool(registry, "llm.rate_limiting.enabled"),
		DefaultConcurrency:       getInt(registry, "llm.rate_limiting.default_concurrency"),
		DefaultQueueSize:         getInt(registry, "llm.rate_limiting.default_queue_size"),
		DefaultRequestsPerMinute: getInt(registry, "llm.rate_limiting.default_requests_per_minute"),
		DefaultTokensPerMinute:   getInt(registry, "llm.rate_limiting.default_tokens_per_minute"),
		DefaultRequestBurst:      getInt(registry, "llm.rate_limiting.default_request_burst"),
		DefaultTokenBurst:        getInt(registry, "llm.rate_limiting.default_token_burst"),
		PerProviderLimits:        buildPerProviderRateLimitOverrides(registry),
	}
	if len(cfg.PerProviderLimits) == 0 {
		cfg.PerProviderLimits = nil
	}
	return cfg
}

func buildPerProviderRateLimitOverrides(registry *definition.Registry) map[string]ProviderRateLimitConfig {
	val := registry.GetDefault("llm.rate_limiting.per_provider_limits")
	if val == nil {
		return nil
	}
	out := make(map[string]ProviderRateLimitConfig)
	switch raw := val.(type) {
	case map[string]ProviderRateLimitConfig:
		for k, v := range raw {
			out[k] = v
		}
	case map[string]any:
		for provider, candidate := range raw {
			m, ok := candidate.(map[string]any)
			if !ok {
				continue
			}
			var cfg ProviderRateLimitConfig
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				TagName:          "mapstructure",
				Result:           &cfg,
				WeaklyTypedInput: true,
			})
			if err != nil {
				continue
			}
			if err := decoder.Decode(m); err != nil {
				continue
			}
			out[provider] = cfg
		}
	default:
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
	prt := getDuration(registry, "cli.port_release_timeout")
	if prt <= 0 {
		prt = DefaultPortReleaseTimeout
	}
	prpi := getDuration(registry, "cli.port_release_poll_interval")
	if prpi <= 0 {
		prpi = DefaultPortReleasePollInterval
	}
	activeWindowDays := getInt(registry, "cli.users.active_window_days")
	if activeWindowDays <= 0 {
		activeWindowDays = DefaultCLIActiveWindowDays
	}
	return CLIConfig{
		APIKey:                  SensitiveString(getString(registry, "cli.api_key")),
		BaseURL:                 getString(registry, "cli.base_url"),
		Timeout:                 getDuration(registry, "cli.timeout"),
		Mode:                    getString(registry, "cli.mode"),
		DefaultFormat:           getString(registry, "cli.default_format"),
		ColorMode:               getString(registry, "cli.color_mode"),
		PageSize:                getInt(registry, "cli.page_size"),
		OutputFormatAlias:       getString(registry, "cli.output_format_alias"),
		NoColor:                 getBool(registry, "cli.no_color"),
		Debug:                   getBool(registry, "cli.debug"),
		Quiet:                   getBool(registry, "cli.quiet"),
		Interactive:             getBool(registry, "cli.interactive"),
		ConfigFile:              getString(registry, "cli.config_file"),
		CWD:                     getString(registry, "cli.cwd"),
		EnvFile:                 getString(registry, "cli.env_file"),
		PortReleaseTimeout:      prt,
		PortReleasePollInterval: prpi,
		MaxRetries:              getInt(registry, "cli.max_retries"),
		Dev:                     buildCLIDevConfig(registry),
		Users: CLIUsersConfig{
			ActiveWindowDays: activeWindowDays,
		},
	}
}

func buildCLIDevConfig(registry *definition.Registry) CLIDevConfig {
	debounce := getDuration(registry, "cli.dev.watcher_debounce")
	if debounce <= 0 {
		debounce = DefaultCLIDevWatcherDebounce
	}
	initial := getDuration(registry, "cli.dev.watcher_retry_initial")
	if initial <= 0 {
		initial = DefaultCLIDevWatcherInitialDelay
	}
	maxDelay := getDuration(registry, "cli.dev.watcher_retry_max")
	if maxDelay <= 0 {
		maxDelay = DefaultCLIDevWatcherMaxDelay
	}
	return CLIDevConfig{
		WatcherDebounce:     debounce,
		WatcherRetryInitial: initial,
		WatcherRetryMax:     maxDelay,
	}
}

func buildRedisConfig(registry *definition.Registry) RedisConfig {
	return RedisConfig{
		Mode:                   getString(registry, "redis.mode"),
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
		Standalone: RedisStandaloneConfig{
			Persistence: RedisPersistenceConfig{
				Enabled:            getBool(registry, "redis.standalone.persistence.enabled"),
				DataDir:            getString(registry, "redis.standalone.persistence.data_dir"),
				SnapshotInterval:   getDuration(registry, "redis.standalone.persistence.snapshot_interval"),
				SnapshotOnShutdown: getBool(registry, "redis.standalone.persistence.snapshot_on_shutdown"),
				RestoreOnStartup:   getBool(registry, "redis.standalone.persistence.restore_on_startup"),
			},
		},
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
		KeyScanCount:         getInt(registry, "cache.key_scan_count"),
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
		StartWorkflowTimeout:       getDuration(registry, "worker.start_workflow_timeout"),
		Dispatcher:                 buildWorkerDispatcherConfig(registry),
	}
}

func buildWorkerDispatcherConfig(registry *definition.Registry) WorkerDispatcherConfig {
	return WorkerDispatcherConfig{
		HeartbeatTTL:   getDuration(registry, "worker.dispatcher.heartbeat_ttl"),
		StaleThreshold: getDuration(registry, "worker.dispatcher.stale_threshold"),
	}
}

func buildMCPProxyConfig(registry *definition.Registry) MCPProxyConfig {
	mode := strings.TrimSpace(getString(registry, "mcp_proxy.mode"))
	port := getInt(registry, "mcp_proxy.port")
	if isEmbeddedMode(mode) && port == 0 {
		port = 6001
	}
	return MCPProxyConfig{
		Mode:                mode,
		Host:                getString(registry, "mcp_proxy.host"),
		Port:                port,
		BaseURL:             getString(registry, "mcp_proxy.base_url"),
		ShutdownTimeout:     getDuration(registry, "mcp_proxy.shutdown_timeout"),
		MaxIdleConns:        getInt(registry, "mcp_proxy.max_idle_conns"),
		MaxIdleConnsPerHost: getInt(registry, "mcp_proxy.max_idle_conns_per_host"),
		MaxConnsPerHost:     getInt(registry, "mcp_proxy.max_conns_per_host"),
		IdleConnTimeout:     getDuration(registry, "mcp_proxy.idle_conn_timeout"),
	}
}

func buildAttachmentsConfig(registry *definition.Registry) AttachmentsConfig {
	return AttachmentsConfig{
		MaxDownloadSizeBytes: getInt64(registry, "attachments.max_download_size_bytes"),
		DownloadTimeout:      getDuration(registry, "attachments.download_timeout"),
		MaxRedirects:         getInt(registry, "attachments.max_redirects"),
		TempDirQuotaBytes:    getInt64(registry, "attachments.temp_dir_quota_bytes"),
		TextPartMaxBytes:     getInt64(registry, "attachments.text_part_max_bytes"),
		PDFExtractMaxChars:   getInt(registry, "attachments.pdf_extract_max_chars"),
		HTTPUserAgent:        getString(registry, "attachments.http_user_agent"),
		MIMEHeadMaxBytes:     getInt(registry, "attachments.mime_head_max_bytes"),
		SSRFStrict:           getBool(registry, "attachments.ssrf_strict"),
		AllowedMIMETypes: AttachmentMIMEAllowlist{
			Image: getStringSlice(registry, "attachments.allowed_mime_types.image"),
			Audio: getStringSlice(registry, "attachments.allowed_mime_types.audio"),
			Video: getStringSlice(registry, "attachments.allowed_mime_types.video"),
			PDF:   getStringSlice(registry, "attachments.allowed_mime_types.pdf"),
		},
	}
}

func buildWebhooksConfig(registry *definition.Registry) WebhooksConfig {
	return WebhooksConfig{
		DefaultMethod:    getString(registry, "webhooks.default_method"),
		DefaultMaxBody:   getInt64(registry, "webhooks.default_max_body"),
		DefaultDedupeTTL: getDuration(registry, "webhooks.default_dedupe_ttl"),
		StripeSkew:       getDuration(registry, "webhooks.stripe_skew"),
	}
}
