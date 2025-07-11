package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/cache"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
)

// WorkflowSourceConfig defines the source location for a workflow file.
//
// **Workflows** are the core building blocks of Compozy projects that define task execution sequences.
type WorkflowSourceConfig struct {
	// Source specifies the path to the workflow YAML file relative to the project root.
	//
	// This file contains the task definitions, agent configurations, and execution flow.
	// Paths can be:
	//   - **Relative**: `"./workflows/data-analysis.yaml"` (recommended)
	//   - **Nested**: `"workflows/pipelines/etl.yaml"`
	//   - **Top-level**: `"main-workflow.yaml"`
	//
	// Best practices:
	//   - Organize workflows by domain or functionality
	//   - Use descriptive names that indicate the workflow's purpose
	//   - Keep related workflows in the same directory
	Source string `json:"source" yaml:"source" mapstructure:"source"`
}

// Config represents main Compozy project configuration.
//
// **A Compozy project** is a declarative configuration that coordinates AI agents, workflows, and tools
// to build complex AI-powered applications. Projects serve as the top-level container that:
//   - **Defines reusable workflows** composed of AI agent tasks
//   - **Configures LLM providers** and model access
//   - **Establishes data schemas** for type-safe operations
//   - **Sets up tool execution** environments and security policies
//   - **Manages performance** through caching, monitoring, and optimization
//
// Projects enable teams to build sophisticated AI applications through YAML configuration
// rather than writing imperative code, making AI workflows more **maintainable** and **collaborative**.
//
// ## Example Project Structure
//
//	my-ai-project/
//	├── compozy.yaml           # Project configuration (this file)
//	├── .env                   # Environment variables
//	├── workflows/             # Workflow definitions
//	│   ├── data-analysis.yaml
//	│   └── content-generation.yaml
//	├── agents/                # Agent configurations (with autoload)
//	│   ├── researcher.yaml
//	│   └── writer.yaml
//	├── tools.ts               # Custom tool implementations
//	├── schemas/               # Data schema definitions
//	│   └── user-input.yaml
//	└── memory/                # Memory resources (with autoload)
//	    └── conversation.yaml
//
// ## Minimal Project Configuration
//
//	name: my-project
//	version: 1.0.0
//	description: My AI project
//	workflows:
//	  - source: ./workflow.yaml
//	models:
//	  - provider: openai
//	    model: gpt-4
//	    api_key: "{{ .env.OPENAI_API_KEY }}"
//
// ## Full Project Configuration
//
//	name: enterprise-ai-system
//	version: 2.1.0
//	description: Multi-agent system for enterprise automation
//	author:
//	  name: AI Team
//	  email: ai@company.com
//	  organization: ACME Corp
//
//	workflows:
//	  - source: ./workflows/customer-support.yaml
//	  - source: ./workflows/data-pipeline.yaml
//
//	models:
//	  - provider: openai
//	    model: gpt-4
//	    api_key: "{{ .env.OPENAI_API_KEY }}"
//	  - provider: anthropic
//	    model: claude-3-opus
//	    api_key: "{{ .env.ANTHROPIC_API_KEY }}"
//
//	runtime:
//	  type: bun
//	  entrypoint: ./tools/index.ts
//	  permissions:
//	    - --allow-read=/data
//	    - --allow-net=api.company.com
//	    - --allow-env=API_KEY,DATABASE_URL
//
//	autoload:
//	  enabled: true
//	  strict: true
//	  include:
//	    - "agents/**/*.yaml"
//	    - "memory/**/*.yaml"
//
//	cache:
//	  url: redis://localhost:6379/0
//	  pool_size: 10
//
//	monitoring:
//	  enabled: true
//	  metrics:
//	    provider: prometheus
//	    endpoint: /metrics
//
//	config:
//	  max_string_length: 52428800  # 50MB
//	  async_token_counter_workers: 20
type Config struct {
	// Name is the unique identifier for this Compozy project.
	//
	// **Requirements**:
	//   - Must be unique within your Compozy installation
	//   - Alphanumeric characters, hyphens, and underscores only
	//   - Cannot start with a number
	//   - Maximum 63 characters
	//
	// - **Examples**: `"customer-support-ai"`, `"data-pipeline"`, `"content-generator"`
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// Version specifies the semantic version of this project configuration.
	//
	// **Format**: Follows [Semantic Versioning 2.0.0](https://semver.org/)
	//   - `MAJOR.MINOR.PATCH` (e.g., `1.2.3`)
	//   - Optional pre-release: `1.0.0-alpha.1`
	//   - Optional build metadata: `1.0.0+20230615`
	Version string `json:"version" yaml:"version" mapstructure:"version"`

	// Description provides a human-readable explanation of the project's purpose and capabilities.
	//
	// **Guidelines**:
	//   - Be specific about what the project does
	//   - Include primary use cases and benefits
	//   - Keep it concise (1-3 sentences)
	//   - Avoid technical jargon for broader understanding
	//
	// **Example**: `"Multi-agent customer support system with automated ticket routing"`
	Description string `json:"description" yaml:"description" mapstructure:"description"`

	// Author information for the project.
	//
	// $ref: inline:#author
	Author core.Author `json:"author" yaml:"author" mapstructure:"author"`

	// Workflows defines the list of workflow files that compose this project's AI capabilities.
	Workflows []*WorkflowSourceConfig `json:"workflows" yaml:"workflows" mapstructure:"workflows"`

	// Models configures the LLM providers and model settings available to this project.
	//
	// $ref: schema://provider
	//
	// **Multi-Model Support**:
	//   - Configure multiple providers for redundancy
	//   - Different models for different tasks (cost/performance optimization)
	//   - Fallback chains for high availability
	//
	// **Supported Providers**:
	//   - OpenAI (GPT-4, GPT-3.5, etc.)
	//   - Anthropic (Claude models)
	//   - Google (Gemini models)
	//   - Groq (Fast inference)
	//   - Ollama (Local models)
	//   - Custom providers via API compatibility
	//
	// **Example**:
	//
	// ```yaml
	//models:
	//  # Primary model for complex reasoning
	//  - provider: openai
	//    model: gpt-4-turbo
	//    api_key: "{{ .env.OPENAI_API_KEY }}"
	//    temperature: 0.7
	//    max_tokens: 4000
	//
	//  # Fallback for cost optimization
	//  - provider: anthropic
	//    model: claude-3-haiku
	//    api_key: "{{ .env.ANTHROPIC_API_KEY }}"
	//
	//  # Local model for sensitive data
	//  - provider: ollama
	//    model: llama2:13b
	//    api_url: http://localhost:11434
	// ```
	Models []*core.ProviderConfig `json:"models" yaml:"models" mapstructure:"models"`

	// Schemas defines the data validation schemas used throughout the project workflows.
	//
	// **Schema Benefits**:
	//   - Type safety for workflow inputs/outputs
	//   - Early error detection and validation
	//   - Self-documenting data contracts
	//   - IDE autocomplete support
	//
	// **Example**:
	//
	// ```yaml
	//schemas:
	//  - id: user-input
	//    schema:
	//      type: object
	//      properties:
	//        name:
	//          type: string
	//          minLength: 1
	//        age:
	//          type: integer
	//          minimum: 0
	//      required: ["name"]
	// ```
	Schemas []schema.Schema `json:"schemas" yaml:"schemas" mapstructure:"schemas"`

	// Opts contains project-wide configuration options for performance tuning and behavior control.
	//
	// $ref: inline:#project-options
	Opts Opts `json:"config" yaml:"config" mapstructure:"config"`

	// Runtime specifies the JavaScript/TypeScript execution environment for custom tools.
	//
	// $ref: inline:#runtime
	Runtime RuntimeConfig `json:"runtime" yaml:"runtime" mapstructure:"runtime"`

	// CacheConfig enables and configures caching for improved performance and cost reduction.
	//
	// $ref: inline:#cache
	CacheConfig *cache.Config `json:"cache,omitempty" yaml:"cache,omitempty" mapstructure:"cache"`

	// AutoLoad configures automatic loading and reloading of project resources during development.
	//
	// $ref: inline:#autoload
	//
	// **Development Benefits**:
	//   - Hot-reload agents and workflows without restart
	//   - Automatic discovery of new resources
	//   - Faster iteration cycles
	//   - Validation on file changes
	//
	// **Example**:
	//
	// ```yaml
	// autoload:
	//   enabled: true
	//   strict: true              # Fail on validation errors
	//   watch_interval: 2s        # Check for changes every 2 seconds
	//   include:
	//     - "agents/**/*.yaml"
	//     - "workflows/**/*.yaml"
	//     - "memory/**/*.yaml"
	//   exclude:
	//     - "**/*.tmp"
	//     - "**/*~"
	// ```
	AutoLoad *autoload.Config `json:"autoload,omitempty" yaml:"autoload,omitempty" mapstructure:"autoload,omitempty"`

	// MonitoringConfig enables observability and metrics collection for performance tracking.
	//
	// $ref: inline:#monitoring
	MonitoringConfig *monitoring.Config `json:"monitoring,omitempty" yaml:"monitoring,omitempty" mapstructure:"monitoring"`

	// filePath stores the absolute path to the configuration file for internal use
	filePath string

	// CWD represents the current working directory context for the project.
	CWD *core.PathCWD `json:"CWD,omitempty" yaml:"CWD,omitempty" mapstructure:"CWD,omitempty"`

	// env stores the loaded environment variables for the project (internal use)
	env *core.EnvMap

	// autoloadValidated caches whether autoload config has been validated (internal use)
	autoloadValidated bool

	// autoloadValidError stores any validation error from autoload config (internal use)
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
		if err := validateRuntimeType(runtime.Type); err != nil {
			return err
		}
	}

	// Validate entrypoint path if specified
	if runtime.Entrypoint != "" {
		if err := validateEntrypointPath(p.CWD, runtime.Entrypoint); err != nil {
			return err
		}
		if err := validateEntrypointExtension(runtime.Entrypoint); err != nil {
			return err
		}
	}

	return nil
}

// validateRuntimeType validates that the runtime type is one of the supported values
func validateRuntimeType(runtimeType string) error {
	validTypes := []string{"bun", "node"}
	if slices.Contains(validTypes, runtimeType) {
		return nil
	}
	return fmt.Errorf(
		"runtime configuration error: invalid runtime type '%s' - supported types are %v",
		runtimeType,
		validTypes,
	)
}

// validateEntrypointPath validates that the entrypoint file exists and is accessible
func validateEntrypointPath(cwd *core.PathCWD, entrypoint string) error {
	if cwd == nil {
		return fmt.Errorf(
			"runtime configuration error: working directory must be set before validating entrypoint path '%s'",
			entrypoint,
		)
	}
	entrypointPath := filepath.Join(cwd.PathStr(), entrypoint)
	if _, err := os.Stat(entrypointPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"runtime configuration error: entrypoint file '%s' does not exist at path '%s'",
			entrypoint,
			entrypointPath,
		)
	} else if err != nil {
		return fmt.Errorf(
			"runtime configuration error: failed to access entrypoint file '%s': %w",
			entrypointPath,
			err,
		)
	}
	return nil
}

// validateEntrypointExtension validates that the entrypoint file has a supported extension
func validateEntrypointExtension(entrypoint string) error {
	ext := filepath.Ext(entrypoint)
	if ext != ".ts" && ext != ".js" {
		return fmt.Errorf(
			"runtime configuration error: entrypoint file '%s' has unsupported extension '%s' - "+
				"supported extensions are .ts and .js",
			entrypoint,
			ext,
		)
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
	setIntConfigFromEnv("MAX_STRING_LENGTH", &config.Opts.MaxStringLength, 10485760, log)
	setIntConfigFromEnv("DISPATCHER_HEARTBEAT_INTERVAL", &config.Opts.DispatcherHeartbeatInterval, 30, log)
	setIntConfigFromEnv("DISPATCHER_HEARTBEAT_TTL", &config.Opts.DispatcherHeartbeatTTL, 300, log)
	setIntConfigFromEnv("DISPATCHER_STALE_THRESHOLD", &config.Opts.DispatcherStaleThreshold, 120, log)
	setIntConfigFromEnv("MAX_MESSAGE_CONTENT_LENGTH", &config.Opts.MaxMessageContentLength, 10240, log)
	setIntConfigFromEnv("MAX_TOTAL_CONTENT_SIZE", &config.Opts.MaxTotalContentSize, 102400, log)
	setIntConfigFromEnv("ASYNC_TOKEN_COUNTER_WORKERS", &config.Opts.AsyncTokenCounterWorkers, 10, log)
	setIntConfigFromEnv("ASYNC_TOKEN_COUNTER_BUFFER_SIZE", &config.Opts.AsyncTokenCounterBufferSize, 1000, log)
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
