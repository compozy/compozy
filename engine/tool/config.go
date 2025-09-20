// Package tool provides configuration and execution management for external tools in Compozy workflows.
// Tools are executable components that extend AI agent capabilities by providing access to
// external systems, APIs, computational resources, and custom business logic.
package tool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/tmc/langchaingo/llms"
	"gopkg.in/yaml.v3"
)

// Config represents a tool configuration in Compozy.
//
// Tools are **executable components** that extend AI agent capabilities by providing
// secure, type-safe access to external systems, APIs, computational resources, and
// custom business logic. They serve as the bridge between AI reasoning and real-world
// system interactions.
//
// ## Core Capabilities
//
// Tools provide essential system integration features:
//
// - **🔒 Type Safety**: JSON Schema validation for inputs and outputs
// - **⚙️ Execution Control**: Configurable timeouts and environment isolation
// - **🤖 LLM Integration**: Automatic function definition generation for AI agents
// - **🌍 Environment Management**: Secure variable passing and working directory control
// - **📊 Schema Validation**: Runtime validation of data flowing through the tool
//
// ## Implementation Types
//
// Tools support multiple execution patterns:
//
// - **JavaScript/TypeScript modules** executed via Bun runtime
// - **External command-line utilities** with process isolation
// - **HTTP API endpoints** for remote service integration
// - **Model Context Protocol (MCP) servers** for advanced tool capabilities
//
// ## Example Configuration
//
// ```yaml
// resource: "tool"
// id: "file-reader"
// description: "Read and parse various file formats with validation"
// timeout: "30s"
//
// input:
//
//	type: "object"
//	properties:
//	  path:
//	    type: "string"
//	    description: "File path to read"
//	    pattern: "^[^\\0]+$"  # Prevent null bytes
//	  format:
//	    type: "string"
//	    enum: ["json", "yaml", "csv", "txt"]
//	    default: "json"
//	required: ["path"]
//
// output:
//
//	type: "object"
//	properties:
//	  content:
//	    type: "string"
//	    description: "File contents"
//	  metadata:
//	    type: "object"
//	    properties:
//	      size:
//	        type: "integer"
//	      modified:
//	        type: "string"
//	        format: "date-time"
//	required: ["content"]
//
// with:
//
//	default_format: "json"
//	max_size_mb: 10
//
// env:
//
//	MAX_FILE_SIZE: "10MB"
//	CACHE_DIR: "/tmp/tool-cache"
//
// ```
type Config struct {
	// Resource identifier for the autoloader system (must be `"tool"`).
	// This field enables automatic discovery and registration of tool configurations.
	Resource string `json:"resource,omitempty"    yaml:"resource,omitempty"    mapstructure:"resource,omitempty"`
	// Unique identifier for the tool within the project scope.
	// Used for referencing the tool in agent configurations, workflows, and function calls.
	// Must be unique across all tools in the project.
	//
	// - **Examples:** `"file-reader"`, `"api-client"`, `"data-processor"`
	// - **Naming:** Use kebab-case for consistency with other Compozy identifiers
	ID string `json:"id,omitempty"          yaml:"id,omitempty"          mapstructure:"id,omitempty"`
	// Human-readable description of the tool's functionality and purpose.
	// This description is used by AI agents to understand when and how to use the tool.
	// Should clearly explain capabilities, limitations, and expected use cases.
	//
	// - **Best practices:** Be specific about what the tool does and its constraints
	// - **Example:** `"Read and parse various file formats including JSON, YAML, and CSV with size limits"`
	Description string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
	// Maximum execution time for the tool in Go duration format.
	// If not specified, uses the global tool timeout from project configuration.
	// This timeout applies to the entire tool execution lifecycle.
	//
	// - **Examples:** `"30s"`, `"5m"`, `"1h"`, `"500ms"`
	// - **Constraints:** Must be positive; zero or negative values cause validation errors
	// - **Default fallback:** Uses project-level tool timeout when empty
	Timeout string `json:"timeout,omitempty"     yaml:"timeout,omitempty"     mapstructure:"timeout,omitempty"`
	// JSON schema defining the expected input parameters for the tool.
	// Used for validation before execution and to generate LLM function call definitions.
	// Must follow JSON Schema Draft 7 specification for compatibility.
	//
	// - **When nil:** Tool accepts any input format (no validation performed)
	// - **Use cases:** Parameter validation, type safety, auto-generated documentation
	// - **Integration:** Automatically converts to LLM function parameters
	InputSchema *schema.Schema `json:"input,omitempty"       yaml:"input,omitempty"       mapstructure:"input,omitempty"`
	// JSON schema defining the expected output format from the tool.
	// Used for validation after execution and documentation purposes.
	// Must follow JSON Schema Draft 7 specification for compatibility.
	//
	// - **When nil:** No output validation is performed
	// - **Use cases:** Response validation, type safety, workflow data flow verification
	// - **Best practice:** Define output schema for tools used in critical workflows
	OutputSchema *schema.Schema `json:"output,omitempty"      yaml:"output,omitempty"      mapstructure:"output,omitempty"`
	// Default input parameters merged with runtime parameters provided by agents.
	// Provides a way to set tool defaults while allowing runtime customization.
	//
	// - **Merge strategy:** Runtime parameters override defaults (shallow merge)
	// - **Use cases:** Default API URLs, fallback configurations, preset options
	// - **Security note:** Avoid storing secrets here; use environment variables instead
	With *core.Input `json:"with,omitempty"        yaml:"with,omitempty"        mapstructure:"with,omitempty"`
	// Configuration parameters passed to the tool separately from input data.
	// Provides static configuration that tools can use for initialization and behavior control.
	// Unlike input parameters, config is not meant to change between tool invocations.
	//
	// - **Use cases:** API base URLs, retry policies, timeout settings, feature flags
	// - **Separation:** Keeps configuration separate from runtime input data
	// - **Override:** Can be overridden at workflow or agent level
	// - **Example:**
	//   ```yaml
	//   config:
	//     base_url: "https://api.example.com"
	//     timeout: 30
	//     retry_count: 3
	//     headers:
	//       User-Agent: "Compozy/1.0"
	//   ```
	Config *core.Input `json:"config,omitempty"      yaml:"config,omitempty"      mapstructure:"config,omitempty"`
	// Environment variables available during tool execution.
	// Variables are isolated to the tool's execution context for security.
	// Used for configuration, API keys, and runtime settings.
	//
	// - **Security:** Variables are only accessible within the tool's execution
	// - **Template support:** Values can use template expressions for dynamic configuration
	// - **Example:**
	//   ```yaml
	//   env:
	//     API_KEY: "{{ .env.SECRET_API_KEY }}"
	//     BASE_URL: "https://api.example.com"
	//     DEBUG: "{{ .project.debug | default(false) }}"
	//   ```
	Env *core.EnvMap `json:"env,omitempty"         yaml:"env,omitempty"         mapstructure:"env,omitempty"`

	// filePath stores the filesystem path where this configuration was loaded from.
	// Used internally for resolving relative paths and debugging.
	filePath string
	// CWD defines the working directory for tool execution.
	// Used for resolving relative file paths and setting process working directory.
	CWD *core.PathCWD
}

// UnmarshalYAML supports both string-form selectors ("tool: \"fmt\"") and
// full object form. When a scalar string is provided, it is interpreted as the
// tool ID selector (ID-only). Object form follows normal decoding.
func (t *Config) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	if value.Kind == yaml.ScalarNode {
		var id string
		if err := value.Decode(&id); err != nil {
			return err
		}
		t.ID = id
		t.Resource = "tool"
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("tool config must be scalar ID or mapping, got kind=%v", value.Kind)
	}
	type alias Config
	var tmp alias
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	*t = Config(tmp)
	if t.Resource == "" {
		t.Resource = "tool"
	}
	return nil
}

// Component returns the configuration type identifier for this tool config.
// Used by the autoloader system to classify and route configurations appropriately.
func (t *Config) Component() core.ConfigType {
	return core.ConfigTool
}

// GetFilePath returns the filesystem path where this tool configuration was loaded from.
// Used for resolving relative paths and providing context in error messages.
func (t *Config) GetFilePath() string {
	return t.filePath
}

// SetFilePath sets the filesystem path where this tool configuration was loaded from.
// Called automatically by the configuration loader during tool discovery.
func (t *Config) SetFilePath(path string) {
	t.filePath = path
}

// SetCWD sets the current working directory for tool execution.
// This directory is used for resolving relative file paths and as the process working directory.
// Returns an error if the path is invalid or cannot be converted to a PathCWD.
func (t *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.CWD = CWD
	return nil
}

// GetCWD returns the working directory for tool execution.
// Used by the runtime system to set the process working directory and resolve relative paths.
func (t *Config) GetCWD() *core.PathCWD {
	return t.CWD
}

// GetEnv returns environment variables for tool execution.
// Creates an empty EnvMap if none is configured. Environment variables are isolated
// to the tool's execution context for security.
func (t *Config) GetEnv() core.EnvMap {
	if t.Env == nil {
		t.Env = &core.EnvMap{}
		return *t.Env
	}
	return *t.Env
}

// GetTimeout returns the tool's configured timeout or the provided global default.
// Used by the runtime system to enforce execution time limits and prevent runaway tools.
// Returns an error if the timeout format is invalid or the value is non-positive.
func (t *Config) GetTimeout(ctx context.Context, globalTimeout time.Duration) (time.Duration, error) {
	if t.Timeout == "" {
		return globalTimeout, nil
	}
	timeout, err := time.ParseDuration(t.Timeout)
	if err != nil {
		logger.FromContext(ctx).Warn(
			"Invalid tool timeout format",
			"tool_id", t.ID,
			"configured_timeout", t.Timeout,
			"error", err,
		)
		return 0, fmt.Errorf("invalid tool timeout '%s': %w", t.Timeout, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("tool timeout must be positive, got: %v", timeout)
	}
	return timeout, nil
}

// GetInput returns the default input parameters configured for this tool.
// These parameters are merged with runtime parameters provided by agents during execution.
// Returns an empty Input if no defaults are configured.
func (t *Config) GetInput() *core.Input {
	if t.With == nil {
		return &core.Input{}
	}
	return t.With
}

// GetConfig returns the configuration parameters for this tool.
// These parameters provide static configuration that tools can use for initialization.
// Returns an empty Input if no configuration is defined.
func (t *Config) GetConfig() *core.Input {
	if t.Config == nil {
		return &core.Input{}
	}
	return t.Config
}

// HasSchema checks if input or output validation is configured for this tool.
// Used to determine whether the runtime should perform schema validation during execution.
// Returns true if either input or output schema is defined.
func (t *Config) HasSchema() bool {
	return t.InputSchema != nil || t.OutputSchema != nil
}

// Validate ensures the tool configuration is valid and complete.
// Performs comprehensive validation of all configuration fields including working directory
// and timeout format. Should be called before using the tool in production workflows.
func (t *Config) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(t.CWD, t.ID),
	)
	if err := v.Validate(); err != nil {
		return err
	}
	if t.Timeout != "" {
		timeout, err := time.ParseDuration(t.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout format '%s': %w", t.Timeout, err)
		}
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive, got: %v", timeout)
		}
	}
	return nil
}

// ValidateInput checks if the provided input conforms to the tool's input schema.
// Used by the runtime system before tool execution to ensure type safety.
// Returns nil if no input schema is configured or if input is nil.
func (t *Config) ValidateInput(ctx context.Context, input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(input, t.InputSchema, t.ID).Validate(ctx)
}

// ValidateOutput checks if the provided output conforms to the tool's output schema.
// Used by the runtime system after tool execution to ensure response integrity.
// Returns nil if no output schema is configured or if output is nil.
func (t *Config) ValidateOutput(ctx context.Context, output *core.Output) error {
	if t.OutputSchema == nil || output == nil {
		return nil
	}
	return schema.NewParamsValidator(output, t.OutputSchema, t.ID).Validate(ctx)
}

// Merge combines another tool configuration into this one using override semantics.
// Used for configuration composition and inheritance patterns.
// The other configuration's fields will override this configuration's fields.
func (t *Config) Merge(other any) error {
	if t == nil {
		return fmt.Errorf("failed to merge tool configs: %w", errors.New("nil config receiver"))
	}
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge tool configs: invalid type for merge: %T", other)
	}
	if otherConfig == nil {
		return nil
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

// Clone creates a deep copy of the tool configuration.
// Used when the configuration needs to be modified without affecting the original.
// Returns nil if the configuration is nil.
func (t *Config) Clone() (*Config, error) {
	if t == nil {
		return nil, nil
	}
	return core.DeepCopy(t)
}

// AsMap serializes the configuration to a map for template processing and storage.
// Used by the template engine and configuration persistence systems.
// Returns a map representation suitable for JSON/YAML serialization.
func (t *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(t)
}

// FromMap deserializes map data and merges it into this configuration.
// Used for loading configurations from template-processed data or external sources.
// The incoming data is merged using the standard merge semantics.
func (t *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return t.Merge(config)
}

// GetLLMDefinition converts the tool configuration to an LLM function definition.
// Used by AI agents to understand how to call the tool through function calling.
// Returns a Tool definition compatible with LangChain Go's LLM interface.
func (t *Config) GetLLMDefinition() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        t.ID,
			Description: t.Description,
			Parameters:  t.InputSchema,
		},
	}
}

// IsTypeScript checks if a file path represents a TypeScript file.
// Used by the runtime system to determine the appropriate execution strategy.
// Returns true for files with .ts or .TS extensions (case-insensitive).
func IsTypeScript(path string) bool {
	return core.IsTypeScript(path)
}

// Load reads and parses a tool configuration from disk.
// The path is resolved relative to the provided working directory.
// Returns the parsed configuration or an error if loading fails.
func Load(cwd *core.PathCWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	return config, nil
}
