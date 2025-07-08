package agent

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
)

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
// ## Key Capabilities
//
// Agents combine several powerful features:
//
// - **LLM Integration**: Connect to various AI providers (OpenAI, Anthropic, etc.)
// - **Action System**: Define structured actions with input/output schemas
// - **Tool Access**: Utilize external tools and APIs dynamically
// - **MCP Support**: Extend capabilities through Model Context Protocol servers
// - **Memory Management**: Access shared context across workflow steps
// - **Iterative Execution**: Self-correct and refine responses over multiple iterations
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
//	model: "claude-3-opus-20240229"
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
	// Resource identifier for the autoloader system (must be `"agent"`)
	// This field enables automatic discovery and registration of agent configurations.
	Resource string `json:"resource,omitempty"       yaml:"resource,omitempty"       mapstructure:"resource,omitempty"`
	// Unique identifier for the agent within the project scope.
	// Used for referencing the agent in workflows and other configurations.
	//
	// **Examples:** `"code-assistant"`, `"data-analyst"`, `"customer-support"`
	ID string `json:"id"                       yaml:"id"                       mapstructure:"id"                       validate:"required"`
	// LLM provider configuration defining which AI model to use and its parameters.
	// Supports multiple providers including OpenAI, Anthropic, Google, and others.
	//
	// **Required fields:**
	// - `provider`: The AI provider name (e.g., `"openai"`, `"anthropic"`)
	// - `model`: The specific model identifier
	// - `params`: Model-specific parameters like temperature, max_tokens
	Config core.ProviderConfig `json:"config"                   yaml:"config"                   mapstructure:"config"                   validate:"required"`
	// System instructions that define the agent's personality, behavior, and constraints.
	// These instructions guide how the agent interprets tasks and generates responses.
	//
	// **Best practices:**
	// - Be clear and specific about the agent's role
	// - Define boundaries and ethical guidelines
	// - Include domain-specific knowledge or constraints
	// - Use markdown formatting for better structure
	Instructions string `json:"instructions"             yaml:"instructions"             mapstructure:"instructions"             validate:"required"`
	// Structured actions the agent can perform with defined input/output schemas.
	// Actions provide type-safe interfaces for specific agent capabilities.
	//
	// Each action includes:
	// - `id`: Unique identifier for the action
	// - `prompt`: Specific instructions for this action
	// - `input`/`output`: JSON schemas for validation
	// - `json_mode`: Whether to enforce JSON output
	Actions []*ActionConfig `json:"actions,omitempty"        yaml:"actions,omitempty"        mapstructure:"actions,omitempty"`
	// Default input parameters passed to the agent on every invocation.
	// These values are merged with runtime inputs, with runtime values taking precedence.
	//
	// **Use cases:**
	// - Setting default configuration values
	// - Providing constant context or settings
	// - Injecting workflow-level parameters
	With *core.Input `json:"with,omitempty"           yaml:"with,omitempty"           mapstructure:"with,omitempty"`
	// Environment variables available during agent execution.
	// Used for configuration, secrets, and runtime settings.
	//
	// **Example:**
	// ```yaml
	// env:
	//   API_KEY: "{{.env.OPENAI_API_KEY}}"
	//   DEBUG_MODE: "true"
	// ```
	Env *core.EnvMap `json:"env,omitempty"            yaml:"env,omitempty"            mapstructure:"env,omitempty"`
	// Tools available to the agent for extending its capabilities.
	// When tools are defined, the agent automatically has `toolChoice` set to `"auto"`,
	// allowing it to decide when and how to use available tools.
	//
	// Tools can be:
	// - File system operations
	// - API integrations
	// - Data processing utilities
	// - Custom business logic
	Tools []tool.Config `json:"tools,omitempty"          yaml:"tools,omitempty"          mapstructure:"tools,omitempty"`
	// Model Context Protocol (MCP) server configurations.
	// MCPs provide standardized interfaces for extending agent capabilities
	// with external services and data sources.
	//
	// **Common MCP types:**
	// - Database connectors
	// - Search engines
	// - Knowledge bases
	// - External APIs
	MCPs []mcp.Config `json:"mcps,omitempty"           yaml:"mcps,omitempty"           mapstructure:"mcps,omitempty"`
	// Maximum number of reasoning iterations the agent can perform.
	// The agent may self-correct and refine its response across iterations.
	//
	// **Default:** `5`
	//
	// **Considerations:**
	// - Higher values allow more thorough problem-solving
	// - Each iteration consumes tokens and adds latency
	// - Set based on task complexity and accuracy requirements
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

	filePath string
	CWD      *core.PathCWD
}

func (a *Config) Component() core.ConfigType {
	return core.ConfigAgent
}

func (a *Config) GetFilePath() string {
	return a.filePath
}

func (a *Config) SetFilePath(path string) {
	a.filePath = path
}

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
	return nil
}

func (a *Config) GetCWD() *core.PathCWD {
	return a.CWD
}

func (a *Config) GetInput() *core.Input {
	if a.With == nil {
		a.With = &core.Input{}
	}
	return a.With
}

func (a *Config) GetEnv() core.EnvMap {
	if a.Env == nil {
		a.Env = &core.EnvMap{}
		return *a.Env
	}
	return *a.Env
}

func (a *Config) HasSchema() bool {
	return false
}

// GetMaxIterations returns the maximum number of iterations for the agent.
// If not explicitly set, defaults to 5 iterations to balance between
// thorough problem-solving and resource consumption.
func (a *Config) GetMaxIterations() int {
	if a.MaxIterations == 0 {
		return 5
	}
	return a.MaxIterations
}

// NormalizeAndValidateMemoryConfig processes and validates the memory configuration.
// It ensures all memory references have required fields (id, key) and valid access modes.
// Sets default mode to "read-write" if not specified.
//
// Returns an error if:
// - Missing required fields (id or key)
// - Invalid access mode (must be "read-write" or "read-only")
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

func (a *Config) ValidateInput(_ context.Context, _ *core.Input) error {
	// Does not make sense the agent having a schema
	return nil
}

func (a *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	// Does not make sense the agent having a schema
	return nil
}

func (a *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: %s", "invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

func (a *Config) Clone() (*Config, error) {
	if a == nil {
		return nil, nil
	}
	return core.DeepCopy(a)
}

func (a *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(a)
}

func (a *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return a.Merge(config)
}

// Load reads and parses an agent configuration from the specified file path.
// The path can be absolute or relative to the provided working directory.
// Supports YAML and JSON formats.
//
// Example:
//
//	config, err := agent.Load(cwd, "agents/code-assistant.yaml")
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

// LoadAndEval loads an agent configuration and evaluates any template expressions.
// This allows for dynamic configuration values using the Compozy template syntax.
//
// Template expressions can reference:
// - Environment variables: {{.env.VARIABLE_NAME}}
// - Workflow inputs: {{.workflow.input.field_name}}
// - Other context values provided by the evaluator
//
// Example:
//
//	evaluator := ref.NewEvaluator(context)
//	config, err := agent.LoadAndEval(cwd, "agents/dynamic-agent.yaml", evaluator)
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
