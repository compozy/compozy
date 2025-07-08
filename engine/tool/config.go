package tool

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/tmc/langchaingo/llms"
)

// Config represents a tool configuration in Compozy.
//
// Tools are executable components that extend AI agent capabilities by providing
// access to external systems, APIs, or computational resources. They define:
//
// - **Input/output schemas** for type safety and validation
// - **Execution parameters** and environment configuration
// - **Timeout controls** and error handling
// - **Integration** with LLM function calling interfaces
//
// ## Implementation Types
//
// Tools can be implemented as:
//
// - **JavaScript/TypeScript modules** for custom logic
// - **External command-line utilities**
// - **HTTP API endpoints**
// - **Model Context Protocol (MCP) servers**
//
// ## Example Configuration
//
//	 resource: "tool"
//	 id: "file-reader"
//	 description: "Read and parse various file formats"
//	 timeout: "30s"
//	 input:
//		type: "object"
//		properties:
//		  path:
//		    type: "string"
//		    description: "File path to read"
//		  format:
//		    type: "string"
//		    enum: ["json", "yaml", "csv", "txt"]
//	 output:
//		type: "object"
//		properties:
//		  content:
//		    type: "string"
//		  metadata:
//		    type: "object"
//	 with:
//		default_format: "json"
//	 env:
//		MAX_FILE_SIZE: "10MB"
type Config struct {
	// Resource identifier for the autoloader system (must be `"tool"`)
	Resource string `json:"resource,omitempty"    yaml:"resource,omitempty"    mapstructure:"resource,omitempty"`
	// Unique identifier for the tool, used in agent configurations and function calls.
	// Must be **unique** within the project scope.
	//
	// - **Examples:** `"file-reader"`, `"api-client"`, `"data-processor"`
	ID string `json:"id,omitempty"          yaml:"id,omitempty"          mapstructure:"id,omitempty"`
	// Human-readable description of what the tool does and its purpose.
	// This description is used by AI agents to understand when to use the tool.
	// Should clearly explain the tool's functionality and expected use cases.
	//
	// - **Example:** `"Read and parse various file formats including JSON, YAML, and CSV"`
	Description string `json:"description,omitempty" yaml:"description,omitempty" mapstructure:"description,omitempty"`
	// Maximum execution time for the tool in **Go duration format**.
	// If not specified, uses the global tool timeout from project configuration.
	//
	// - **Examples:** `"30s"`, `"5m"`, `"1h"`, `"500ms"`
	// > **Note:** Zero or negative values are invalid and will cause validation errors
	Timeout string `json:"timeout,omitempty"     yaml:"timeout,omitempty"     mapstructure:"timeout,omitempty"`
	// JSON schema defining the expected input parameters for the tool.
	// Used for validation and to generate LLM function call definitions.
	// Should follow **JSON Schema Draft 7** specification.
	//
	// If `nil`, the tool accepts any input format.
	InputSchema *schema.Schema `json:"input,omitempty"       yaml:"input,omitempty"       mapstructure:"input,omitempty"`
	// JSON schema defining the expected output format from the tool.
	// Used for validation and documentation purposes.
	// Should follow **JSON Schema Draft 7** specification.
	//
	// If `nil`, no output validation is performed.
	OutputSchema *schema.Schema `json:"output,omitempty"      yaml:"output,omitempty"      mapstructure:"output,omitempty"`
	// Default input parameters to pass to the tool.
	// These values are merged with runtime parameters provided by agents.
	//
	// - **Precedence:** Runtime parameters take precedence over default values.
	// - **Use case:** Setting default configurations or API keys.
	With *core.Input `json:"with,omitempty"        yaml:"with,omitempty"        mapstructure:"with,omitempty"`
	// Environment variables available during tool execution.
	// Used for configuration, API keys, and runtime settings.
	// Variables are isolated to the tool's execution context.
	//
	// **Example:**
	// ```yaml
	// env:
	//   API_KEY: "secret"
	//   BASE_URL: "https://api.example.com"
	// ```
	Env *core.EnvMap `json:"env,omitempty"         yaml:"env,omitempty"         mapstructure:"env,omitempty"`

	filePath string
	CWD      *core.PathCWD
}

// Component returns the configuration type identifier for this tool config
func (t *Config) Component() core.ConfigType {
	return core.ConfigTool
}

// GetFilePath returns the file path where this tool configuration was loaded from
func (t *Config) GetFilePath() string {
	return t.filePath
}

// SetFilePath sets the file path where this tool configuration was loaded from
func (t *Config) SetFilePath(path string) {
	t.filePath = path
}

// SetCWD sets the current working directory for the tool
func (t *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	t.CWD = CWD
	return nil
}

// GetCWD returns the current working directory for tool execution
func (t *Config) GetCWD() *core.PathCWD {
	return t.CWD
}

// GetEnv returns the environment variables for the tool
// If no environment is configured, returns an empty environment map
func (t *Config) GetEnv() core.EnvMap {
	if t.Env == nil {
		t.Env = &core.EnvMap{}
		return *t.Env
	}
	return *t.Env
}

// GetTimeout returns the tool-specific timeout with fallback to global timeout
func (t *Config) GetTimeout(globalTimeout time.Duration) (time.Duration, error) {
	if t.Timeout == "" {
		return globalTimeout, nil
	}
	timeout, err := time.ParseDuration(t.Timeout)
	if err != nil {
		// Log warning for debugging
		// Note: We can't get activity context here, so using context.Background()
		logger.FromContext(context.Background()).Warn(
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

// GetInput returns the default input parameters for the tool
// If no default input is configured, returns an empty input object
func (t *Config) GetInput() *core.Input {
	if t.With == nil {
		return &core.Input{}
	}
	return t.With
}

// HasSchema returns true if the tool has input or output schema validation configured
func (t *Config) HasSchema() bool {
	return t.InputSchema != nil || t.OutputSchema != nil
}

// Validate validates the tool configuration
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

// ValidateInput validates the provided input against the tool's input schema
// Returns nil if no input schema is configured or if validation passes
func (t *Config) ValidateInput(ctx context.Context, input *core.Input) error {
	if t.InputSchema == nil || input == nil {
		return nil
	}
	return schema.NewParamsValidator(input, t.InputSchema, t.ID).Validate(ctx)
}

// ValidateOutput validates the provided output against the tool's output schema
// Returns nil if no output schema is configured or if validation passes
func (t *Config) ValidateOutput(ctx context.Context, output *core.Output) error {
	if t.OutputSchema == nil || output == nil {
		return nil
	}
	return schema.NewParamsValidator(output, t.OutputSchema, t.ID).Validate(ctx)
}

// Merge merges another tool configuration into this one using override semantics
// Fields from the other configuration will override corresponding fields in this configuration
func (t *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge tool configs: %w", errors.New("invalid type for merge"))
	}
	return mergo.Merge(t, otherConfig, mergo.WithOverride)
}

// Clone creates a deep copy of this tool configuration
// Returns nil if the source configuration is nil
func (t *Config) Clone() (*Config, error) {
	if t == nil {
		return nil, nil
	}
	return core.DeepCopy(t)
}

// AsMap converts the tool configuration to a map[string]any representation
// Useful for serialization and template processing
func (t *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(t)
}

// FromMap populates the tool configuration from a map[string]any representation
// Merges the provided data with the existing configuration
func (t *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return t.Merge(config)
}

// GetLLMDefinition converts the tool configuration to an LLM function definition
// Used by AI agents to understand how to call the tool through function calling
// The input schema is used as the function parameters definition
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

// IsTypeScript returns true if the given file path has a TypeScript extension (.ts)
// Used to determine the appropriate execution runtime for tool implementations
func IsTypeScript(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".ts")
}

// Load loads a tool configuration from the specified file path
// The path is resolved relative to the provided current working directory
// Returns an error if the file cannot be found or parsed
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

// LoadAndEval loads a tool configuration from the specified file path with template evaluation
// Templates in the configuration are evaluated using the provided evaluator
// Useful for dynamic tool configurations based on runtime context
func LoadAndEval(cwd *core.PathCWD, path string, ev *ref.Evaluator) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfigWithEvaluator[*Config](filePath, ev)
	if err != nil {
		return nil, err
	}
	return config, nil
}
