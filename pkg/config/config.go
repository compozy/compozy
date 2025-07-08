package config

import (
	"context"
	"time"

	"github.com/compozy/compozy/pkg/config/definition"
)

// Config represents the complete configuration for the Compozy system.
// It provides type-safe access to all configuration values with validation.
type Config struct {
	Server   ServerConfig   `koanf:"server"   validate:"required"`
	Database DatabaseConfig `koanf:"database" validate:"required"`
	Temporal TemporalConfig `koanf:"temporal" validate:"required"`
	Runtime  RuntimeConfig  `koanf:"runtime"  validate:"required"`
	Limits   LimitsConfig   `koanf:"limits"   validate:"required"`
	OpenAI   OpenAIConfig   `koanf:"openai"`
	Memory   MemoryConfig   `koanf:"memory"`
	LLM      LLMConfig      `koanf:"llm"`
}

// ServerConfig contains HTTP server configuration.
type ServerConfig struct {
	Host        string        `koanf:"host"         validate:"required"        env:"SERVER_HOST"`
	Port        int           `koanf:"port"         validate:"min=1,max=65535" env:"SERVER_PORT"`
	CORSEnabled bool          `koanf:"cors_enabled"                            env:"SERVER_CORS_ENABLED"`
	Timeout     time.Duration `koanf:"timeout"                                 env:"SERVER_TIMEOUT"`
}

// DatabaseConfig contains database connection configuration.
type DatabaseConfig struct {
	ConnString string          `koanf:"conn_string" env:"DB_CONN_STRING"`
	Host       string          `koanf:"host"        env:"DB_HOST"`
	Port       string          `koanf:"port"        env:"DB_PORT"`
	User       string          `koanf:"user"        env:"DB_USER"`
	Password   SensitiveString `koanf:"password"    env:"DB_PASSWORD"    sensitive:"true"`
	DBName     string          `koanf:"name"        env:"DB_NAME"`
	SSLMode    string          `koanf:"ssl_mode"    env:"DB_SSL_MODE"`
}

// TemporalConfig contains Temporal workflow engine configuration.
type TemporalConfig struct {
	HostPort  string `koanf:"host_port"  env:"TEMPORAL_HOST_PORT"`
	Namespace string `koanf:"namespace"  env:"TEMPORAL_NAMESPACE"`
	TaskQueue string `koanf:"task_queue" env:"TEMPORAL_TASK_QUEUE"`
}

// RuntimeConfig contains runtime behavior configuration.
type RuntimeConfig struct {
	Environment                 string        `koanf:"environment"                     validate:"oneof=development staging production" env:"RUNTIME_ENVIRONMENT"`
	LogLevel                    string        `koanf:"log_level"                       validate:"oneof=debug info warn error"          env:"RUNTIME_LOG_LEVEL"`
	DispatcherHeartbeatInterval time.Duration `koanf:"dispatcher_heartbeat_interval"                                                   env:"RUNTIME_DISPATCHER_HEARTBEAT_INTERVAL"`
	DispatcherHeartbeatTTL      time.Duration `koanf:"dispatcher_heartbeat_ttl"                                                        env:"RUNTIME_DISPATCHER_HEARTBEAT_TTL"`
	DispatcherStaleThreshold    time.Duration `koanf:"dispatcher_stale_threshold"                                                      env:"RUNTIME_DISPATCHER_STALE_THRESHOLD"`
	AsyncTokenCounterWorkers    int           `koanf:"async_token_counter_workers"     validate:"min=1"                                env:"RUNTIME_ASYNC_TOKEN_COUNTER_WORKERS"`
	AsyncTokenCounterBufferSize int           `koanf:"async_token_counter_buffer_size" validate:"min=1"                                env:"RUNTIME_ASYNC_TOKEN_COUNTER_BUFFER_SIZE"`
}

// LimitsConfig contains system limits and constraints.
type LimitsConfig struct {
	MaxNestingDepth       int `koanf:"max_nesting_depth"        validate:"min=1" env:"LIMITS_MAX_NESTING_DEPTH"`
	MaxStringLength       int `koanf:"max_string_length"        validate:"min=1" env:"LIMITS_MAX_STRING_LENGTH"`
	MaxMessageContent     int `koanf:"max_message_content"      validate:"min=1" env:"LIMITS_MAX_MESSAGE_CONTENT_LENGTH"`
	MaxTotalContentSize   int `koanf:"max_total_content_size"   validate:"min=1" env:"LIMITS_MAX_TOTAL_CONTENT_SIZE"`
	MaxTaskContextDepth   int `koanf:"max_task_context_depth"   validate:"min=1" env:"LIMITS_MAX_TASK_CONTEXT_DEPTH"`
	ParentUpdateBatchSize int `koanf:"parent_update_batch_size" validate:"min=1" env:"LIMITS_PARENT_UPDATE_BATCH_SIZE"`
}

// OpenAIConfig contains OpenAI API configuration.
type OpenAIConfig struct {
	APIKey       SensitiveString `koanf:"api_key"       env:"OPENAI_API_KEY"       sensitive:"true"`
	BaseURL      string          `koanf:"base_url"      env:"OPENAI_BASE_URL"`
	OrgID        string          `koanf:"org_id"        env:"OPENAI_ORG_ID"`
	DefaultModel string          `koanf:"default_model" env:"OPENAI_DEFAULT_MODEL"`
}

// MemoryConfig contains memory service configuration.
type MemoryConfig struct {
	RedisURL    string        `koanf:"redis_url"    env:"MEMORY_REDIS_URL"`
	RedisPrefix string        `koanf:"redis_prefix" env:"MEMORY_REDIS_PREFIX"`
	TTL         time.Duration `koanf:"ttl"          env:"MEMORY_TTL"`
	MaxEntries  int           `koanf:"max_entries"  env:"MEMORY_MAX_ENTRIES"  validate:"min=1"`
}

// LLMConfig contains LLM service configuration.
type LLMConfig struct {
	ProxyURL   string          `koanf:"proxy_url"   env:"MCP_PROXY_URL"`
	AdminToken SensitiveString `koanf:"admin_token" env:"MCP_ADMIN_TOKEN" sensitive:"true"`
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
		Server: ServerConfig{
			Host:        getString(registry, "server.host"),
			Port:        getInt(registry, "server.port"),
			CORSEnabled: getBool(registry, "server.cors_enabled"),
			Timeout:     getDuration(registry, "server.timeout"),
		},
		Database: DatabaseConfig{
			Host:     getString(registry, "database.host"),
			Port:     getString(registry, "database.port"),
			User:     getString(registry, "database.user"),
			Password: SensitiveString(getString(registry, "database.password")),
			DBName:   getString(registry, "database.name"),
			SSLMode:  getString(registry, "database.ssl_mode"),
		},
		Temporal: TemporalConfig{
			HostPort:  getString(registry, "temporal.host_port"),
			Namespace: getString(registry, "temporal.namespace"),
			TaskQueue: getString(registry, "temporal.task_queue"),
		},
		Runtime: RuntimeConfig{
			Environment:                 getString(registry, "runtime.environment"),
			LogLevel:                    getString(registry, "runtime.log_level"),
			DispatcherHeartbeatInterval: getDuration(registry, "runtime.dispatcher_heartbeat_interval"),
			DispatcherHeartbeatTTL:      getDuration(registry, "runtime.dispatcher_heartbeat_ttl"),
			DispatcherStaleThreshold:    getDuration(registry, "runtime.dispatcher_stale_threshold"),
			AsyncTokenCounterWorkers:    getInt(registry, "runtime.async_token_counter_workers"),
			AsyncTokenCounterBufferSize: getInt(registry, "runtime.async_token_counter_buffer_size"),
		},
		Limits: LimitsConfig{
			MaxNestingDepth:       getInt(registry, "limits.max_nesting_depth"),
			MaxStringLength:       getInt(registry, "limits.max_string_length"),
			MaxMessageContent:     getInt(registry, "limits.max_message_content"),
			MaxTotalContentSize:   getInt(registry, "limits.max_total_content_size"),
			MaxTaskContextDepth:   getInt(registry, "limits.max_task_context_depth"),
			ParentUpdateBatchSize: getInt(registry, "limits.parent_update_batch_size"),
		},
		OpenAI: OpenAIConfig{
			APIKey:       SensitiveString(getString(registry, "openai.api_key")),
			BaseURL:      getString(registry, "openai.base_url"),
			OrgID:        getString(registry, "openai.org_id"),
			DefaultModel: getString(registry, "openai.default_model"),
		},
		Memory: MemoryConfig{
			RedisURL:    getString(registry, "memory.redis_url"),
			RedisPrefix: getString(registry, "memory.redis_prefix"),
			TTL:         getDuration(registry, "memory.ttl"),
			MaxEntries:  getInt(registry, "memory.max_entries"),
		},
		LLM: LLMConfig{
			ProxyURL:   getString(registry, "llm.proxy_url"),
			AdminToken: SensitiveString(getString(registry, "llm.admin_token")),
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
