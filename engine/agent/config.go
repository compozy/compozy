// Package agent provides AI agent configuration and management for Compozy workflows.
// Agents combine LLM capabilities with structured actions, tools, and memory to solve
// complex tasks through iterative reasoning and decision-making.
package agent

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
)

type LLMProperties struct {
	// Tools available to the agent for extending its capabilities.
	// When tools are defined, the agent automatically has `toolChoice` set to `"auto"`,
	// enabling autonomous tool selection and invocation during task execution.
	//
	// **Tool types supported:**
	// - File system operations (read, write, list)
	// - API integrations (HTTP requests, webhooks)
	// - Data processing utilities (parsing, transformation)
	// - Custom business logic (TypeScript/JavaScript execution)
	//
	// Tools are referenced by ID and can be shared across multiple agents.
	Tools []tool.Config `json:"tools,omitempty"          yaml:"tools,omitempty"          mapstructure:"tools,omitempty"`
	// Model Context Protocol (MCP) server configurations.
	// MCPs provide standardized interfaces for extending agent capabilities
	// with external services and data sources through protocol-based communication.
	//
	// **Common MCP integrations:**
	// - Database connectors (PostgreSQL, Redis, MongoDB)
	// - Search engines (Elasticsearch, Solr)
	// - Knowledge bases (vector databases, documentation systems)
	// - External APIs (REST, GraphQL, gRPC services)
	//
	// MCPs support both stdio and HTTP transport protocols.
	MCPs []mcp.Config `json:"mcps,omitempty"           yaml:"mcps,omitempty"           mapstructure:"mcps,omitempty"`
	// Maximum number of reasoning iterations the agent can perform.
	// The agent may self-correct and refine its response across multiple iterations
	// to improve accuracy and address complex multi-step problems.
	//
	// **Default:** `5` iterations
	//
	// **Trade-offs:**
	// - Higher values enable more thorough problem-solving and self-correction
	// - Each iteration consumes additional tokens and increases response latency
	// - Configure based on task complexity, accuracy requirements, and cost constraints
	MaxIterations int `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty" mapstructure:"max_iterations,omitempty"`
	// Forces the agent to always respond in valid JSON format.
	// When enabled, the agent's responses must be parseable JSON objects.
	//
	// **Use cases:**
	// - API integrations requiring structured data
	// - Automated processing of agent outputs
	// - Ensuring consistent response formats
	//
	// ⚠️ **Note:** May limit the agent's ability to provide explanatory text
	JSONMode bool `json:"json_mode"                yaml:"json_mode"                mapstructure:"json_mode"`
	// Memory references enabling the agent to access persistent context.
	// Memory provides stateful interactions across workflow steps and sessions.
	//
	// **Configuration format:**
	// ```yaml
	// memory:
	//   - id: "user_context"           # Memory resource ID
	//     key: "user:{{.user_id}}"     # Dynamic key with template
	//     mode: "read-write"           # Access mode (default: "read-write")
	// ```
	//
	// **Access modes:**
	// - `"read-write"`: Full access to read and modify memory
	// - `"read-only"`: Can only read existing memory entries
	Memory []core.MemoryReference `json:"memory,omitempty"         yaml:"memory,omitempty"         mapstructure:"memory,omitempty"`
}

// Config represents an AI agent configuration in Compozy.
//
// Agents are **autonomous AI-powered entities** that can reason, make decisions,
// and execute actions based on natural language instructions. They serve as the
// intelligent core of Compozy workflows, orchestrating complex tasks through:
//
// - **Natural language understanding** and contextual reasoning
// - **Dynamic action selection** based on goals and constraints
// - **Tool utilization** for extending capabilities
// - **Iterative problem-solving** with self-correction
// - **Memory integration** for maintaining context across interactions
//
// ## Architecture Integration
//
// Agents integrate seamlessly with Compozy's workflow engine:
//
// - **LLM Providers**: Support OpenAI, Anthropic, Google, Groq, and local models
// - **Action System**: Type-safe interfaces with JSON Schema validation
// - **Tool Ecosystem**: Dynamic access to filesystem, APIs, and custom tools
// - **MCP Protocol**: Extensible capabilities through external servers
// - **Memory Layer**: Persistent context across workflow sessions
// - **Workflow Tasks**: Execute as part of basic, parallel, or collection tasks
//
// ## Example Configuration
//
// ```yaml
// resource: "agent"
// id: "code-assistant"
//
// config:
//
//	provider: "anthropic"
//	model: "claude-4-opus"
//	params:
//	  temperature: 0.7
//	  max_tokens: 4000
//
// instructions: |
//
//	You are an expert software engineer specializing in code review.
//	Focus on clarity, performance, and best practices.
//	Always explain your reasoning and provide actionable feedback.
//
// actions:
//   - id: "review-code"
//     prompt: "Analyze code for quality and improvements"
//     input:
//     type: "object"
//     properties:
//     code:
//     type: "string"
//     json_mode: true
//
// tools:
//   - resource: "tool"
//     id: "file-reader"
//
// memory:
//   - id: "conversation_history"
//     key: "session:{{.workflow.session_id}}"
//     mode: "read-write"
//
// max_iterations: 10
// json_mode: false
// ```
type Config struct {
	// Embed LLMProperties with inline tags for backward compatibility
	// This allows fields to be accessed directly on Config in YAML/JSON
	LLMProperties `json:",inline" yaml:",inline" mapstructure:",squash"`

	// Attachments declared at the agent scope.
	Attachments attachment.Attachments `json:"attachments,omitempty" yaml:"attachments,omitempty" mapstructure:"attachments,omitempty"`

	// Resource identifier for the autoloader system (must be `"agent"`).
	// This field enables automatic discovery and registration of agent configurations.
	Resource string `json:"resource,omitempty" yaml:"resource,omitempty" mapstructure:"resource,omitempty"`
	// Unique identifier for the agent within the project scope.
	// Used for referencing the agent in workflows and other configurations.
	//
	// - **Examples:** `"code-assistant"`, `"data-analyst"`, `"customer-support"`
	ID string `json:"id"                 yaml:"id"                 mapstructure:"id"                 validate:"required"`
	// LLM provider configuration defining which AI model to use and its parameters.
	// Supports multiple providers including OpenAI, Anthropic, Google, Groq, and local models.
	//
	// **Required fields:** provider, model
	// **Optional fields:** api_key, api_url, params (temperature, max_tokens, etc.)
	Config core.ProviderConfig `json:"config"             yaml:"config"             mapstructure:"config"             validate:"required"`
	// System instructions that define the agent's personality, behavior, and constraints.
	// These instructions guide how the agent interprets tasks and generates responses.
	//
	// **Best practices:**
	// - Be clear and specific about the agent's role
	// - Define boundaries and ethical guidelines
	// - Include domain-specific knowledge or constraints
	// - Use markdown formatting for better structure
	Instructions string `json:"instructions"       yaml:"instructions"       mapstructure:"instructions"       validate:"required"`
	// Structured actions the agent can perform with defined input/output schemas.
	// Actions provide type-safe interfaces for specific agent capabilities.
	//
	// **Example:**
	// ```yaml
	// actions:
	//   - id: "review-code"
	//     prompt: |
	//       Analyze code {{.input.code}} for quality and improvements
	//     json_mode: true
	//     input:
	//       type: "object"
	//       properties:
	//         code:
	//           type: "string"
	//           description: "The code to review"
	//     output:
	//       type: "object"
	//       properties:
	//         quality:
	//           type: "string"
	//           description: "The quality of the code"
	// ```
	//
	// $ref: inline:#action-configuration
	Actions []*ActionConfig `json:"actions,omitempty"  yaml:"actions,omitempty"  mapstructure:"actions,omitempty"`
	// Default input parameters passed to the agent on every invocation.
	// These values are merged with runtime inputs, with runtime values taking precedence.
	//
	// **Use cases:**
	// - Setting default configuration values
	// - Providing constant context or settings
	// - Injecting workflow-level parameters
	With *core.Input `json:"with,omitempty"     yaml:"with,omitempty"     mapstructure:"with,omitempty"`
	// Environment variables available during agent execution.
	// Used for configuration, secrets, and runtime settings.
	//
	// **Example:**
	// ```yaml
	// env:
	//   API_KEY: "{{.env.OPENAI_API_KEY}}"
	//   DEBUG_MODE: "true"
	// ```
	Env *core.EnvMap `json:"env,omitempty"      yaml:"env,omitempty"      mapstructure:"env,omitempty"`

	filePath string
	CWD      *core.PathCWD
}

// Component returns the configuration type identifier for agents.
// This method implements the core.Config interface and is used by the
// configuration system to identify agent configurations during processing.
func (a *Config) Component() core.ConfigType {
	return core.ConfigAgent
}

// GetFilePath returns the source file path of this configuration.
func (a *Config) GetFilePath() string {
	return a.filePath
}

// SetFilePath sets the source file path for this configuration.
func (a *Config) SetFilePath(path string) {
	a.filePath = path
}

// SetCWD sets the working directory and propagates it to all actions.
// This ensures that all relative paths in the agent configuration and its actions
// are resolved consistently from the same base directory.
func (a *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	a.CWD = CWD
	for i := range a.Actions {
		if err := a.Actions[i].SetCWD(path); err != nil {
			return err
		}
	}
	// Propagate CWD to inline tool configurations as well. Agent-level tools
	// are validated later when creating the LLM service (via resolved tools),
	// and require a non-empty CWD for path resolution and execution. When tools
	// are declared at workflow/project level, CWD propagation happens there;
	// for agent-scoped tools we must set it here to avoid validation failures.
	for i := range a.Tools {
		if err := a.Tools[i].SetCWD(path); err != nil {
			return err
		}
	}
	return nil
}

// GetCWD returns the current working directory.
func (a *Config) GetCWD() *core.PathCWD {
	return a.CWD
}

// GetInput returns the default input configuration, creating one if needed.
func (a *Config) GetInput() *core.Input {
	if a.With == nil {
		a.With = &core.Input{}
	}
	return a.With
}

// GetEnv returns the environment variables map, creating one if needed.
func (a *Config) GetEnv() core.EnvMap {
	if a.Env == nil {
		a.Env = &core.EnvMap{}
		return *a.Env
	}
	return *a.Env
}

// HasSchema indicates whether this configuration supports schema validation.
// Returns false for agents since they operate on dynamic natural language prompts
// rather than structured input/output schemas like tools.
func (a *Config) HasSchema() bool {
	return false
}

// GetMaxIterations returns the maximum iteration count, defaulting to 5.
// This provides a safe default for agents that don't explicitly configure
// iteration limits, balancing thoroughness with performance.
func (a *Config) GetMaxIterations() int {
	if a.MaxIterations == 0 {
		return 5
	}
	return a.MaxIterations
}

// NormalizeAndValidateMemoryConfig validates memory references and sets default mode to "read-write".
// This ensures all memory configurations have valid IDs, keys, and access modes before agent execution.
func (a *Config) NormalizeAndValidateMemoryConfig() error {
	const defaultMemoryMode = "read-write"

	for i := range a.Memory {
		if a.Memory[i].ID == "" {
			return fmt.Errorf("memory reference %d missing required 'id' field", i)
		}
		if a.Memory[i].Key == "" {
			return fmt.Errorf("memory reference %d (id: %s) missing required 'key' field", i, a.Memory[i].ID)
		}
		if a.Memory[i].Mode == "" {
			a.Memory[i].Mode = defaultMemoryMode
		}
		if a.Memory[i].Mode != "read-write" && a.Memory[i].Mode != "read-only" {
			return fmt.Errorf(
				"memory reference %d (id: %s) has invalid mode '%s', must be 'read-write' or 'read-only'",
				i, a.Memory[i].ID, a.Memory[i].Mode,
			)
		}
	}
	return nil
}

// Validate ensures the agent configuration is complete and correct.
// This performs comprehensive validation including struct fields, memory configuration,
// actions, and MCP server settings to prevent runtime errors.
func (a *Config) Validate() error {
	// Initial struct validation (for required fields like ID, Config, Instructions)
	baseValidator := schema.NewStructValidator(a)
	if err := baseValidator.Validate(); err != nil {
		return err
	}

	// Normalize and validate memory configuration first
	if err := a.NormalizeAndValidateMemoryConfig(); err != nil {
		return fmt.Errorf("invalid memory configuration: %w", err)
	}

	// Now build composite validator including memory (if any)
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.CWD, a.ID),
		NewActionsValidator(a.Actions),
		NewMemoryValidator(a.Memory),
	)
	if err := v.Validate(); err != nil {
		return fmt.Errorf("agent config validation failed: %w", err)
	}

	var mcpErrors []error
	for i := range a.MCPs {
		if err := a.MCPs[i].Validate(); err != nil {
			mcpErrors = append(mcpErrors, fmt.Errorf("mcp validation error: %w", err))
		}
	}
	if len(mcpErrors) > 0 {
		return errors.Join(mcpErrors...)
	}
	return nil
}

// ValidateInput is a no-op for agents as they don't have input schemas.
// Agents accept dynamic natural language inputs that cannot be validated against
// predefined schemas, unlike tools which have structured input requirements.
func (a *Config) ValidateInput(_ context.Context, _ *core.Input) error {
	return nil
}

// ValidateOutput is a no-op for agents as they don't have output schemas.
// Agents generate dynamic natural language outputs that cannot be validated against
// predefined schemas, unlike tools which have structured output formats.
func (a *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	return nil
}

// Merge combines this configuration with another, with the other taking precedence.
// This enables configuration inheritance and composition patterns where base agent
// configurations can be extended or overridden by more specific configurations.
func (a *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

// Clone creates a deep copy of the configuration.
// This is useful for creating independent configuration instances when
// multiple agents need similar but not identical configurations.
func (a *Config) Clone() (*Config, error) {
	if a == nil {
		return nil, nil
	}
	return core.DeepCopy(a)
}

// AsMap converts the configuration to a map representation.
// This is primarily used for serialization and template evaluation processes.
func (a *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(a)
}

// FromMap populates the configuration from a map representation.
// This is used during configuration loading and template evaluation to
// reconstruct agent configurations from deserialized data.
func (a *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return a.Merge(config)
}

// Load reads an agent configuration from a YAML or JSON file.
// This function resolves the file path relative to the provided working directory
// and loads the configuration without template evaluation.
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

// LoadAndEval loads a configuration with template evaluation support.
// This function processes template expressions like {{.env.API_KEY}} and {{.workflow.input.value}}
// before loading the agent configuration, enabling dynamic configuration patterns.
// LoadAndEval has been removed. Use Load() and the compile/link step instead.
