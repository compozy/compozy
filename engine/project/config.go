package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/autoload"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
)

// RuntimeConfig defines project-specific runtime overrides.
// The main runtime configuration is now in pkg/config.RuntimeConfig.
// This struct allows projects to override specific runtime settings.
type RuntimeConfig struct {
	// Type specifies the JavaScript runtime to use for tool execution.
	// Overrides global runtime.runtime_type setting if specified.
	Type string `json:"type,omitempty" yaml:"type,omitempty" mapstructure:"type"`

	// Entrypoint specifies the path to the JavaScript/TypeScript entrypoint file.
	// Overrides global runtime.entrypoint_path setting if specified.
	Entrypoint string `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty" mapstructure:"entrypoint"`

	// Permissions defines runtime security permissions.
	// Overrides global runtime.bun_permissions setting if specified.
	Permissions []string `json:"permissions,omitempty" yaml:"permissions,omitempty" mapstructure:"permissions"`

	// ToolExecutionTimeout overrides the global runtime.tool_execution_timeout when provided.
	// Accepts Go duration strings (e.g., "120s", "2m").
	ToolExecutionTimeout time.Duration `json:"tool_execution_timeout,omitempty" yaml:"tool_execution_timeout,omitempty" mapstructure:"tool_execution_timeout"`
}

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
//	  entrypoint: ./tools.ts
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
	// NOTE: Runtime configuration has been moved to global config (pkg/config.RuntimeConfig)
	// This field is kept for backwards compatibility and project-specific overrides.
	//
	// $ref: schema://application#runtime
	Runtime RuntimeConfig `json:"runtime" yaml:"runtime" mapstructure:"runtime"`

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

	// Tools defines shared tool definitions available to all workflows and agents
	// within this project. These tools are inherited unless explicitly overridden.
	//
	// **Inheritance Rules**:
	//   - Agent tools completely override inheritance when present
	//   - Workflow tools override project tools by ID
	//   - Tool ID collisions resolved by precedence: Agent > Workflow > Project
	//
	// **Location & autoload**:
	//   - Place reusable tool configuration files under the `tools/` directory (e.g., `tools/*.yaml`)
	//   - If autoload is enabled, files in `tools/` will be discovered and validated automatically
	//
	// **Example**:
	//
	// ```yaml
	// tools:
	//   - id: code-analyzer
	//     description: Analyzes code quality and patterns
	//     timeout: 30s
	//   - id: data-processor
	//     description: Processes and transforms data
	// ```
	Tools []tool.Config `json:"tools,omitempty" yaml:"tools,omitempty" mapstructure:"tools,omitempty"`

	// Memories declares project-scoped memory resources that agents and tasks can reference
	// by ID. These are indexed into the ResourceStore under the current project and can be
	// used across workflows for conversation and state sharing.
	//
	// Example:
	//
	//  memories:
	//    - id: conversation
	//      type: buffer
	//      persistence:
	//        type: in_memory
	//
	// The Resource field on memory.Config is optional in project-level definitions and will
	// default to "memory" during validation.
	Memories []memory.Config `json:"memories,omitempty" yaml:"memories,omitempty" mapstructure:"memories,omitempty"`

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

// GetDefaultModel returns the model configuration marked as default.
// Returns nil if no model is marked as default.
// This is used as a fallback when tasks or agents don't specify a model.
func (p *Config) GetDefaultModel() *core.ProviderConfig {
	if p.Models == nil {
		return nil
	}
	for _, model := range p.Models {
		if model != nil && model.Default {
			return model
		}
	}
	return nil
}

func (p *Config) Validate() error {
	validator := schema.NewCompositeValidator(
		schema.NewCWDValidator(p.CWD, p.Name),
		// TODO: Add workflows validator back in
		// NewWorkflowsValidator(p.Workflows),
	)
	if err := validator.Validate(); err != nil {
		return err
	}
	// Validate models configuration (including default model)
	if err := p.validateModels(); err != nil {
		return err
	}
	// Validate runtime configuration
	if err := p.validateRuntimeConfig(); err != nil {
		return fmt.Errorf("runtime configuration validation failed: %w", err)
	}
	// Validate project-level tools
	if err := p.validateTools(); err != nil {
		return fmt.Errorf("project tools validation failed: %w", err)
	}
	// Validate project-level memories
	if err := p.validateMemories(); err != nil {
		return fmt.Errorf("project memories validation failed: %w", err)
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
	if p.Opts.SourceOfTruth != "" {
		m := strings.ToLower(strings.TrimSpace(p.Opts.SourceOfTruth))
		if m != "repo" && m != "builder" {
			return fmt.Errorf(
				"project configuration error: opts.source_of_truth must be 'repo' or 'builder', got '%s'",
				p.Opts.SourceOfTruth,
			)
		}
		p.Opts.SourceOfTruth = m
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

// validateModels ensures that at most one model is marked as default
func (p *Config) validateModels() error {
	if len(p.Models) == 0 {
		return nil
	}
	firstIdx := -1
	for i, model := range p.Models {
		if model != nil && model.Default {
			if firstIdx == -1 {
				firstIdx = i
			} else {
				return fmt.Errorf(
					"project configuration error: only one model can be marked as default, found multiple at indices %d and %d",
					firstIdx,
					i,
				)
			}
		}
	}
	return nil
}

// validateTools validates the project-level tools configuration
func (p *Config) validateTools() error {
	if len(p.Tools) == 0 {
		return nil
	}
	toolIDs := make(map[string]struct{}, len(p.Tools))
	for i := range p.Tools {
		// Validate tool configuration
		if err := p.Tools[i].Validate(); err != nil {
			return fmt.Errorf("tool[%d] validation failed: %w", i, err)
		}
		// Check for duplicate tool IDs
		if p.Tools[i].ID == "" {
			return fmt.Errorf("tool[%d] missing required ID field", i)
		}
		if _, exists := toolIDs[p.Tools[i].ID]; exists {
			return fmt.Errorf("duplicate tool ID '%s' found in project tools", p.Tools[i].ID)
		}
		toolIDs[p.Tools[i].ID] = struct{}{}
	}
	return nil
}

// validateMemories validates project-level memory resources declared inline.
// It normalizes missing Resource fields to "memory" for parity with autoloaded files
// and REST validators, enforces unique IDs, and applies memory.Config.Validate().
func (p *Config) validateMemories() error {
	if len(p.Memories) == 0 {
		return nil
	}
	ids := make(map[string]struct{}, len(p.Memories))
	for i := range p.Memories {
		// Default resource kind for inline definitions
		if strings.TrimSpace(p.Memories[i].Resource) == "" {
			p.Memories[i].Resource = string(core.ConfigMemory)
		}
		if p.Memories[i].ID == "" {
			return fmt.Errorf("memory[%d] missing required ID field", i)
		}
		if _, ok := ids[p.Memories[i].ID]; ok {
			return fmt.Errorf("duplicate memory ID '%s' found in project memories", p.Memories[i].ID)
		}
		// Reuse memory.Config.Validate to ensure parity with REST validation
		if err := p.Memories[i].Validate(); err != nil {
			return fmt.Errorf("memory[%d] validation failed: %w", i, err)
		}
		ids[p.Memories[i].ID] = struct{}{}
	}
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

	// Validate tool execution timeout if specified
	if runtime.ToolExecutionTimeout < 0 {
		return fmt.Errorf("runtime configuration error: tool_execution_timeout must be non-negative if specified")
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
	config, _, err := core.LoadConfig[*Config](ctx, filePath)
	if err != nil {
		return nil, err
	}
	if config.CWD == nil {
		config.CWD = cwd
	}
	if config.AutoLoad != nil {
		config.AutoLoad.SetDefaults()
	}
	// Apply minimal runtime defaults for project-level compatibility
	config.setRuntimeDefaults()
	config.MonitoringConfig, err = monitoring.LoadWithEnv(ctx, config.MonitoringConfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// setRuntimeDefaults applies minimal runtime defaults for project-level compatibility
// These defaults ensure backward compatibility with existing tests and expected behavior
func (p *Config) setRuntimeDefaults() {
	// When runtime config is not specified in YAML, provide standard defaults
	if p.Runtime.Type == "" {
		p.Runtime.Type = "bun"
	}
	if p.Runtime.Entrypoint == "" {
		p.Runtime.Entrypoint = "./tools.ts"
	}
	// Set default permissions for project runtime if not specified
	// Use nil check to distinguish between unspecified (nil) and explicitly empty ([]string{})
	if p.Runtime.Permissions == nil {
		p.Runtime.Permissions = []string{"--allow-read"}
	}
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
	return config, nil
}

func (p *Config) SetDefaultModel(agent *agent.Config) {
	if agent == nil || p == nil {
		return
	}
	if agent.Model.Config.Provider == "" || agent.Model.Config.Model == "" {
		if def := p.GetDefaultModel(); def != nil {
			if agent.Model.Config.Provider == "" {
				agent.Model.Config.Provider = def.Provider
			}
			if agent.Model.Config.Model == "" {
				agent.Model.Config.Model = def.Model
			}
			if agent.Model.Config.APIKey == "" {
				agent.Model.Config.APIKey = def.APIKey
			}
			if agent.Model.Config.APIURL == "" {
				agent.Model.Config.APIURL = def.APIURL
			}
			if agent.Model.Config.Organization == "" {
				agent.Model.Config.Organization = def.Organization
			}
			copyPromptParams(&agent.Model.Config.Params, &def.Params)
		}
	}
}

func copyPromptParams(dst, src *core.PromptParams) {
	if dst == nil || src == nil {
		return
	}
	copyPromptMaxTokens(dst, src)
	copyPromptTemperature(dst, src)
	copyPromptStopWords(dst, src)
	copyPromptTopK(dst, src)
	copyPromptTopP(dst, src)
	copyPromptSeed(dst, src)
	copyPromptMinLength(dst, src)
	copyPromptRepetitionPenalty(dst, src)
}

func copyPromptMaxTokens(dst, src *core.PromptParams) {
	if !dst.IsSetMaxTokens() && src.IsSetMaxTokens() {
		dst.SetMaxTokens(src.MaxTokens)
	}
}

func copyPromptTemperature(dst, src *core.PromptParams) {
	if !dst.IsSetTemperature() && src.IsSetTemperature() {
		dst.SetTemperature(src.Temperature)
	}
}

func copyPromptStopWords(dst, src *core.PromptParams) {
	if !dst.IsSetStopWords() && src.IsSetStopWords() {
		cloned := append([]string(nil), src.StopWords...)
		dst.SetStopWords(cloned)
	}
}

func copyPromptTopK(dst, src *core.PromptParams) {
	if !dst.IsSetTopK() && src.IsSetTopK() {
		dst.SetTopK(src.TopK)
	}
}

func copyPromptTopP(dst, src *core.PromptParams) {
	if !dst.IsSetTopP() && src.IsSetTopP() {
		dst.SetTopP(src.TopP)
	}
}

func copyPromptSeed(dst, src *core.PromptParams) {
	if !dst.IsSetSeed() && src.IsSetSeed() {
		dst.SetSeed(src.Seed)
	}
}

func copyPromptMinLength(dst, src *core.PromptParams) {
	if !dst.IsSetMinLength() && src.IsSetMinLength() {
		dst.SetMinLength(src.MinLength)
	}
}

func copyPromptRepetitionPenalty(dst, src *core.PromptParams) {
	if !dst.IsSetRepetitionPenalty() && src.IsSetRepetitionPenalty() {
		dst.SetRepetitionPenalty(src.RepetitionPenalty)
	}
}
