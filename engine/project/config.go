package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
)

type WorkflowSourceConfig struct {
	Source string `json:"source" yaml:"source" mapstructure:"source"`
}

// RuntimeConfig defines the JavaScript runtime configuration for tool execution.
// This configuration controls which runtime environment is used and how tools are executed.
// Security Note: These settings directly impact the security posture of tool execution.
type RuntimeConfig struct {
	// Type specifies the JavaScript runtime to use for tool execution.
	// Valid values: "bun", "node"
	// Default: "bun" (if not specified)
	// Security: Different runtimes have different security models and capabilities
	Type string `json:"type,omitempty" yaml:"type,omitempty" mapstructure:"type"`

	// Entrypoint specifies the path to the JavaScript/TypeScript file that exports all available tools.
	// This file serves as the single entry point for tool discovery and execution.
	// The path is relative to the project root directory.
	// Required file extensions: .ts (TypeScript) or .js (JavaScript)
	// Default: "./tools.ts" (if not specified)
	// Security: Must be a trusted file as it has access to all tool implementations
	Entrypoint string `json:"entrypoint" yaml:"entrypoint" mapstructure:"entrypoint"`

	// Permissions defines the security permissions granted to the runtime during tool execution.
	// These permissions control what system resources tools can access.
	// For Bun runtime: Uses Bun's permission flags (e.g., "--allow-read", "--allow-net", "--allow-write")
	// For Node.js runtime: Reserved for future Node.js permission system
	// Default for Bun: ["--allow-read"] (minimal read-only access)
	// Security Critical: Granting excessive permissions can lead to security vulnerabilities.
	// Principle of least privilege should be applied - only grant permissions that tools actually need.
	// Examples:
	//   - ["--allow-read"] - Read-only file system access (recommended minimum)
	//   - ["--allow-read", "--allow-net"] - Read access + network access
	//   - ["--allow-read", "--allow-write", "--allow-net"] - Full access (use with caution)
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty" mapstructure:"permissions"`
}

type Opts struct {
	core.GlobalOpts             `    json:",inline"                                 yaml:",inline"                                 mapstructure:",squash"`
	MaxNestingDepth             int `json:"max_nesting_depth,omitempty"             yaml:"max_nesting_depth,omitempty"             mapstructure:"max_nesting_depth"`
	DispatcherHeartbeatInterval int `json:"dispatcher_heartbeat_interval,omitempty" yaml:"dispatcher_heartbeat_interval,omitempty" mapstructure:"dispatcher_heartbeat_interval"`
	DispatcherHeartbeatTTL      int `json:"dispatcher_heartbeat_ttl,omitempty"      yaml:"dispatcher_heartbeat_ttl,omitempty"      mapstructure:"dispatcher_heartbeat_ttl"`
	DispatcherStaleThreshold    int `json:"dispatcher_stale_threshold,omitempty"    yaml:"dispatcher_stale_threshold,omitempty"    mapstructure:"dispatcher_stale_threshold"`
	MaxMessageContentLength     int `json:"max_message_content_length,omitempty"    yaml:"max_message_content_length,omitempty"    mapstructure:"max_message_content_length"`
	MaxTotalContentSize         int `json:"max_total_content_size,omitempty"        yaml:"max_total_content_size,omitempty"        mapstructure:"max_total_content_size"`
}

type Config struct {
	Name             string                  `json:"name"                 yaml:"name"                 mapstructure:"name"`
	Version          string                  `json:"version"              yaml:"version"              mapstructure:"version"`
	Description      string                  `json:"description"          yaml:"description"          mapstructure:"description"`
	Author           core.Author             `json:"author"               yaml:"author"               mapstructure:"author"`
	Workflows        []*WorkflowSourceConfig `json:"workflows"            yaml:"workflows"            mapstructure:"workflows"`
	Models           []*core.ProviderConfig  `json:"models"               yaml:"models"               mapstructure:"models"`
	Schemas          []schema.Schema         `json:"schemas"              yaml:"schemas"              mapstructure:"schemas"`
	Opts             Opts                    `json:"config"               yaml:"config"               mapstructure:"config"`
	Runtime          RuntimeConfig           `json:"runtime"              yaml:"runtime"              mapstructure:"runtime"`
	CacheConfig      *cache.Config           `json:"cache,omitempty"      yaml:"cache,omitempty"      mapstructure:"cache"`
	AutoLoad         *autoload.Config        `json:"autoload,omitempty"   yaml:"autoload,omitempty"   mapstructure:"autoload,omitempty"`
	MonitoringConfig *monitoring.Config      `json:"monitoring,omitempty" yaml:"monitoring,omitempty" mapstructure:"monitoring"`

	filePath           string
	CWD                *core.PathCWD `json:"CWD,omitempty" yaml:"CWD,omitempty" mapstructure:"CWD,omitempty"`
	env                *core.EnvMap
	autoloadValidated  bool
	autoloadValidError error
}

func (p *Config) Component() core.ConfigType {
	return core.ConfigProject
}

func (p *Config) GetFilePath() string {
	return p.filePath
}

func (p *Config) SetFilePath(path string) {
	p.filePath = path
}

func (p *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	p.CWD = CWD
	return nil
}

func (p *Config) GetCWD() *core.PathCWD {
	return p.CWD
}

func (p *Config) HasSchema() bool {
	return false
}

func (p *Config) Validate() error {
	validator := schema.NewCompositeValidator(
		schema.NewCWDValidator(p.CWD, p.Name),
	)
	if err := validator.Validate(); err != nil {
		return err
	}
	// Validate runtime configuration
	if err := p.validateRuntimeConfig(); err != nil {
		return fmt.Errorf("runtime configuration validation failed: %w", err)
	}
	// Validate cache configuration if present
	if p.CacheConfig != nil {
		if err := p.CacheConfig.Validate(); err != nil {
			return fmt.Errorf("cache configuration validation failed: %w", err)
		}
	}
	// Validate monitoring configuration if present
	if p.MonitoringConfig != nil {
		if err := p.MonitoringConfig.Validate(); err != nil {
			return fmt.Errorf("monitoring configuration validation failed: %w", err)
		}
	}
	// Validate autoload configuration if present (with caching)
	if p.AutoLoad != nil {
		if !p.autoloadValidated {
			p.autoloadValidError = p.AutoLoad.Validate()
			p.autoloadValidated = true
		}
		if p.autoloadValidError != nil {
			return fmt.Errorf("autoload configuration validation failed: %w", p.autoloadValidError)
		}
	}

	return nil
}

func (p *Config) ValidateInput(_ context.Context, _ *core.Input) error {
	return nil
}

func (p *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	// Does not make sense the project having a schema
	return nil
}

// validateRuntimeConfig validates the runtime configuration fields with detailed error messages
func (p *Config) validateRuntimeConfig() error {
	runtime := &p.Runtime

	// Validate runtime type if specified
	if runtime.Type != "" {
		validTypes := []string{"bun", "node"}
		valid := false
		for _, validType := range validTypes {
			if runtime.Type == validType {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf(
				"runtime configuration error: invalid runtime type '%s' - supported types are %v",
				runtime.Type,
				validTypes,
			)
		}
	}

	// Validate entrypoint path if specified
	if runtime.Entrypoint != "" {
		if p.CWD == nil {
			return fmt.Errorf(
				"runtime configuration error: working directory must be set before validating entrypoint path '%s'",
				runtime.Entrypoint,
			)
		}

		// Check if entrypoint path exists
		entrypointPath := filepath.Join(p.CWD.PathStr(), runtime.Entrypoint)
		if _, err := os.Stat(entrypointPath); os.IsNotExist(err) {
			return fmt.Errorf(
				"runtime configuration error: entrypoint file '%s' does not exist at path '%s'",
				runtime.Entrypoint,
				entrypointPath,
			)
		} else if err != nil {
			return fmt.Errorf(
				"runtime configuration error: failed to access entrypoint file '%s': %w",
				entrypointPath,
				err,
			)
		}

		// Validate entrypoint file extension (should be .ts or .js)
		ext := filepath.Ext(runtime.Entrypoint)
		if ext != ".ts" && ext != ".js" {
			return fmt.Errorf(
				"runtime configuration error: entrypoint file '%s' has unsupported extension '%s'",
				runtime.Entrypoint,
				ext,
			)
		}
	}

	return nil
}

func (p *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge project configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(p, otherConfig, mergo.WithOverride)
}

func (p *Config) LoadID() (string, error) {
	return p.Name, nil
}

func (p *Config) loadEnv(envFilePath string) (core.EnvMap, error) {
	if p.CWD == nil {
		return nil, fmt.Errorf("working directory not set for project %q", p.Name)
	}
	env, err := core.NewEnvFromFile(p.CWD.PathStr(), envFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}
	return env, nil
}

func (p *Config) SetEnv(env core.EnvMap) {
	p.env = &env
}

func (p *Config) GetEnv() core.EnvMap {
	if p.env == nil {
		return core.EnvMap{}
	}
	return *p.env
}

func (p *Config) GetInput() *core.Input {
	return &core.Input{}
}

func (p *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(p)
}

func (p *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return p.Merge(config)
}

func (p *Config) Clone() (*Config, error) {
	if p == nil {
		return nil, nil
	}
	return core.DeepCopy(p)
}

// setIntConfigFromEnv sets an integer configuration value from environment variable if valid
func setIntConfigFromEnv(envKey string, currentValue *int, defaultValue int, log logger.Logger) {
	if *currentValue <= 0 {
		*currentValue = defaultValue
	}
	if envValue := os.Getenv(envKey); envValue != "" {
		if envInt, err := strconv.Atoi(envValue); err == nil {
			if envInt > 0 {
				*currentValue = envInt
			} else {
				log.Warn("Invalid environment variable",
					"key", envKey, "value", envValue,
					"error", "must be positive integer",
					"using", *currentValue)
			}
		} else {
			log.Warn("Invalid environment variable",
				"key", envKey, "value", envValue,
				"error", err, "using", *currentValue)
		}
	}
}

// setRuntimeDefaults sets secure and sensible default values for runtime configuration.
// These defaults follow the principle of least privilege and prioritize security over convenience.
func setRuntimeDefaults(runtime *RuntimeConfig) {
	// Default to Bun runtime if not specified
	// Rationale: Bun is chosen as the default because:
	// - Superior performance compared to Node.js for most tool execution scenarios
	// - Built-in TypeScript support without additional compilation steps
	// - More comprehensive permission system for security isolation
	// - Faster startup times which improves tool execution latency
	if runtime.Type == "" {
		runtime.Type = "bun"
	}

	// Set default entrypoint if not specified
	// Rationale: "./tools.ts" is chosen because:
	// - TypeScript provides better type safety and development experience
	// - Relative path ensures entrypoint is within project directory (security)
	// - Conventional name that's intuitive for developers
	// - Single entrypoint simplifies tool discovery and management
	if runtime.Entrypoint == "" {
		runtime.Entrypoint = "./tools.ts"
	}

	// Set default permissions for Bun if not specified
	// Rationale: Only read permissions are granted by default because:
	// - Principle of least privilege - tools should only get minimum required access
	// - Read-only access prevents accidental or malicious file modifications
	// - Network and write permissions can be explicitly granted when needed
	// - Reduces attack surface for potentially vulnerable or malicious tools
	// - Forces developers to consciously evaluate permission requirements
	if len(runtime.Permissions) == 0 && runtime.Type == "bun" {
		runtime.Permissions = []string{
			"--allow-read", // Minimal read-only access for maximum security
		}
	}
}

// configureDispatcherOptions sets dispatcher-related configuration options from environment
func configureDispatcherOptions(config *Config, log logger.Logger) {
	setIntConfigFromEnv("MAX_NESTING_DEPTH", &config.Opts.MaxNestingDepth, 20, log)
	setIntConfigFromEnv("DISPATCHER_HEARTBEAT_INTERVAL", &config.Opts.DispatcherHeartbeatInterval, 30, log)
	setIntConfigFromEnv("DISPATCHER_HEARTBEAT_TTL", &config.Opts.DispatcherHeartbeatTTL, 300, log)
	setIntConfigFromEnv("DISPATCHER_STALE_THRESHOLD", &config.Opts.DispatcherStaleThreshold, 120, log)
	setIntConfigFromEnv("MAX_MESSAGE_CONTENT_LENGTH", &config.Opts.MaxMessageContentLength, 10240, log)
	setIntConfigFromEnv("MAX_TOTAL_CONTENT_SIZE", &config.Opts.MaxTotalContentSize, 102400, log)
}

// loadAndPrepareConfig loads and prepares the configuration file
func loadAndPrepareConfig(ctx context.Context, cwd *core.PathCWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	if config.CWD == nil {
		config.CWD = cwd
	}
	if config.AutoLoad != nil {
		config.AutoLoad.SetDefaults()
	}
	// Set default runtime configuration if not specified
	setRuntimeDefaults(&config.Runtime)
	config.MonitoringConfig, err = monitoring.LoadWithEnv(ctx, config.MonitoringConfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func Load(ctx context.Context, cwd *core.PathCWD, path string, envFilePath string) (*Config, error) {
	config, err := loadAndPrepareConfig(ctx, cwd, path)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	env, err := config.loadEnv(envFilePath)
	if err != nil {
		return nil, err
	}
	config.SetEnv(env)
	log := logger.FromContext(ctx)
	configureDispatcherOptions(config, log)
	return config, nil
}
