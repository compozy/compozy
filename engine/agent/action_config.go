package agent

import (
	"context"
	"fmt"

	"dario.cat/mergo"
	"github.com/compozy/compozy/engine/attachment"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

// ActionConfig defines a structured action that an agent can perform.
//
// Actions provide **type-safe interfaces** for specific agent capabilities,
// enabling predictable and validated interactions. Each action represents
// a discrete task with well-defined inputs and outputs.
//
// ## Key Features
//
// - **Structured prompts** for consistent agent behavior
// - **JSON Schema validation** for inputs and outputs
// - **Isolated execution context** with specific instructions
// - **JSON mode enforcement** for machine-readable responses
//
// ## Example Configuration
//
// ```yaml
// actions:
//   - id: "analyze-code"
//     prompt: |
//     Analyze the provided code for:
//     1. Security vulnerabilities
//     2. Performance bottlenecks
//     3. Code quality issues
//     Return findings in structured format.
//     input:
//     type: "object"
//     properties:
//     code:
//     type: "string"
//     description: "Source code to analyze"
//     language:
//     type: "string"
//     enum: ["python", "go", "javascript"]
//     required: ["code", "language"]
//     output:
//     type: "object"
//     properties:
//     issues:
//     type: "array"
//     items:
//     type: "object"
//     properties:
//     severity:
//     type: "string"
//     enum: ["critical", "high", "medium", "low"]
//     category:
//     type: "string"
//     description:
//     type: "string"
//     json_mode: true
//
// ```
type ActionConfig struct {
	// Unique identifier for the action within the agent's scope.
	// Used to invoke specific actions programmatically.
	//
	// - **Examples:** `"analyze-code"`, `"generate-summary"`, `"validate-data"`
	ID string `json:"id"               yaml:"id"               mapstructure:"id"`
	// Detailed instructions for the agent when executing this action.
	// Should clearly define the expected behavior, output format, and any constraints.
	//
	// **Best practices:**
	// - Be specific about the desired outcome
	// - Include examples if complex formatting is required
	// - Define clear success criteria
	// - Specify any limitations or boundaries
	Prompt string `json:"prompt"           yaml:"prompt"           mapstructure:"prompt"           validate:"required"`
	// JSON Schema defining the expected input parameters for this action.
	// Enables validation and type checking of inputs before execution.
	//
	// If `nil`, the action accepts any input format without validation.
	//
	// **Schema format:** JSON Schema Draft 7
	InputSchema *schema.Schema `json:"input,omitempty"  yaml:"input,omitempty"  mapstructure:"input,omitempty"`
	// JSON Schema defining the expected output format from this action.
	// Used for validating agent responses and ensuring consistent output structure.
	//
	// If `nil`, no output validation is performed.
	//
	// **Schema format:** JSON Schema Draft 7
	OutputSchema *schema.Schema `json:"output,omitempty" yaml:"output,omitempty" mapstructure:"output,omitempty"`
	// Default parameters to provide to the action.
	// These are merged with runtime parameters, with runtime values taking precedence.
	//
	// **Use cases:**
	// - Setting default configuration options
	// - Providing constant context values
	// - Pre-filling common parameters
	With *core.Input `json:"with,omitempty"   yaml:"with,omitempty"   mapstructure:"with,omitempty"`
	// Forces JSON-formatted output for this specific action.
	// When `true`, the agent must return valid JSON that conforms to the output schema.
	//
	// **Note:** If an `OutputSchema` is defined, JSON mode is automatically enabled.
	//
	// ⚠️ **Trade-off:** Enabling JSON mode may limit the agent's ability to provide
	// explanatory text or reasoning alongside the structured output.
	JSONMode bool `json:"json_mode"        yaml:"json_mode"        mapstructure:"json_mode"`
	CWD      *core.PathCWD

	// Attachments at action scope
	Attachments attachment.Attachments `json:"attachments,omitempty" yaml:"attachments,omitempty" mapstructure:"attachments,omitempty"`
}

func (a *ActionConfig) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	a.CWD = CWD
	return nil
}

func (a *ActionConfig) GetCWD() *core.PathCWD {
	return a.CWD
}

func (a *ActionConfig) GetInput() *core.Input {
	if a.With == nil {
		return &core.Input{}
	}
	return a.With
}

func (a *ActionConfig) Validate() error {
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.CWD, a.ID),
		schema.NewStructValidator(a),
	)
	return v.Validate()
}

func (a *ActionConfig) ValidateInput(ctx context.Context, input *core.Input) error {
	return schema.NewParamsValidator(input, a.InputSchema, a.ID).Validate(ctx)
}

func (a *ActionConfig) ValidateOutput(ctx context.Context, output *core.Output) error {
	return schema.NewParamsValidator(output, a.OutputSchema, a.ID).Validate(ctx)
}

// AsMap converts the action configuration to a map for template normalization
func (a *ActionConfig) AsMap() (map[string]any, error) {
	return core.AsMapDefault(a)
}

// FromMap updates the action configuration from a normalized map
func (a *ActionConfig) FromMap(data any) error {
	config, err := core.FromMapDefault[ActionConfig](data)
	if err != nil {
		return err
	}
	return mergo.Merge(a, config, mergo.WithOverride)
}

func (a *ActionConfig) HasSchema() bool {
	return a.InputSchema != nil || a.OutputSchema != nil
}

func (a *ActionConfig) ShouldUseJSONOutput() bool {
	return a.JSONMode || a.OutputSchema != nil
}

func (a *ActionConfig) Clone() (*ActionConfig, error) {
	if a == nil {
		return nil, nil
	}
	return core.DeepCopy(a)
}

// FindActionConfig searches for an action configuration by ID within a slice of actions.
// Returns the matching ActionConfig or an error if not found.
//
// This is commonly used when executing specific agent actions by ID reference.
func FindActionConfig(actions []*ActionConfig, id string) (*ActionConfig, error) {
	for _, action := range actions {
		if action.ID == id {
			return action, nil
		}
	}
	return nil, fmt.Errorf("action config not found: %s", id)
}
