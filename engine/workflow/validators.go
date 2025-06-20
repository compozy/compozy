package workflow

import (
	"context"
	"fmt"
	"text/template"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
)

// -----------------------------------------------------------------------------
// WorkflowValidator - Main validator for workflow configuration
// -----------------------------------------------------------------------------

type Validator struct {
	config *Config
}

func NewWorkflowValidator(config *Config) *Validator {
	return &Validator{config: config}
}

func (v *Validator) Validate() error {
	validator := schema.NewCompositeValidator(
		schema.NewCWDValidator(v.config.CWD, v.config.ID),
		NewTasksValidator(v.config),
		NewAgentsValidator(v.config),
		NewToolsValidator(v.config),
		NewMCPsValidator(v.config),
		NewTriggersValidator(v.config),
		NewOutputsValidator(v.config),
		NewScheduleValidator(v.config),
	)
	return validator.Validate()
}

// -----------------------------------------------------------------------------
// TasksValidator - Validates workflow tasks
// -----------------------------------------------------------------------------

type TasksValidator struct {
	config *Config
}

func NewTasksValidator(config *Config) *TasksValidator {
	return &TasksValidator{config: config}
}

func (v *TasksValidator) Validate() error {
	for i := range v.config.Tasks {
		tc := &v.config.Tasks[i]
		if err := tc.Validate(); err != nil {
			return fmt.Errorf("task validation error: %s", err)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// AgentsValidator - Validates workflow agents
// -----------------------------------------------------------------------------

type AgentsValidator struct {
	config *Config
}

func NewAgentsValidator(config *Config) *AgentsValidator {
	return &AgentsValidator{config: config}
}

func (v *AgentsValidator) Validate() error {
	for i := range v.config.Agents {
		ac := &v.config.Agents[i]
		if err := ac.Validate(); err != nil {
			return fmt.Errorf("agent validation error: %s", err)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// ToolsValidator - Validates workflow tools
// -----------------------------------------------------------------------------

type ToolsValidator struct {
	config *Config
}

func NewToolsValidator(config *Config) *ToolsValidator {
	return &ToolsValidator{config: config}
}

func (v *ToolsValidator) Validate() error {
	for i := range v.config.Tools {
		tc := &v.config.Tools[i]
		if err := tc.Validate(); err != nil {
			return fmt.Errorf("tool validation error: %s", err)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// MCPsValidator - Validates workflow MCPs
// -----------------------------------------------------------------------------

type MCPsValidator struct {
	config *Config
}

func NewMCPsValidator(config *Config) *MCPsValidator {
	return &MCPsValidator{config: config}
}

func (v *MCPsValidator) Validate() error {
	for i := range v.config.MCPs {
		mc := &v.config.MCPs[i]
		if err := mc.Validate(); err != nil {
			return fmt.Errorf("mcp validation error: %s", err)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// TriggersValidator - Validates workflow triggers
// -----------------------------------------------------------------------------

type TriggersValidator struct {
	config *Config
}

func NewTriggersValidator(config *Config) *TriggersValidator {
	return &TriggersValidator{config: config}
}

func (v *TriggersValidator) Validate() error {
	seen := map[string]struct{}{}
	for i := range v.config.Triggers {
		trigger := &v.config.Triggers[i]
		if trigger.Type != TriggerTypeSignal {
			return fmt.Errorf("unsupported trigger type: %s", trigger.Type)
		}
		if trigger.Name == "" {
			return fmt.Errorf("trigger name is required")
		}
		if _, dup := seen[trigger.Name]; dup {
			return fmt.Errorf("duplicate trigger name: %s", trigger.Name)
		}
		seen[trigger.Name] = struct{}{}
		if trigger.Schema != nil {
			if _, err := trigger.Schema.Compile(); err != nil {
				return fmt.Errorf("invalid trigger schema for %s: %w", trigger.Name, err)
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// OutputsValidator - Validates workflow outputs
// -----------------------------------------------------------------------------

type OutputsValidator struct {
	config *Config
}

func NewOutputsValidator(config *Config) *OutputsValidator {
	return &OutputsValidator{config: config}
}

func (v *OutputsValidator) Validate() error {
	if v.config.Outputs == nil {
		return nil
	}
	if len(*v.config.Outputs) == 0 {
		return fmt.Errorf("outputs cannot be empty when defined")
	}
	return v.validateOutputTemplates(*v.config.Outputs, "")
}

func (v *OutputsValidator) validateOutputTemplates(data map[string]any, prefix string) error {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch val := value.(type) {
		case string:
			if err := v.validateTemplateString(val); err != nil {
				return fmt.Errorf("invalid template in outputs.%s: %w", fullKey, err)
			}
		case map[string]any:
			if err := v.validateOutputTemplates(val, fullKey); err != nil {
				return err
			}
		case []any:
			for i, elem := range val {
				if err := v.validateOutputElement(elem, fmt.Sprintf("%s[%d]", fullKey, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (v *OutputsValidator) validateOutputElement(elem any, path string) error {
	switch val := elem.(type) {
	case string:
		if err := v.validateTemplateString(val); err != nil {
			return fmt.Errorf("invalid template in outputs.%s: %w", path, err)
		}
	case map[string]any:
		if err := v.validateOutputTemplates(val, path); err != nil {
			return err
		}
	case []any:
		for i, nestedElem := range val {
			if err := v.validateOutputElement(nestedElem, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *OutputsValidator) validateTemplateString(tmpl string) error {
	_, err := template.New("validation").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}
	return nil
}

// -----------------------------------------------------------------------------
// InputValidator - Validates workflow input against schema
// -----------------------------------------------------------------------------

type InputValidator struct {
	config *Config
	input  *core.Input
}

func NewInputValidator(config *Config, input *core.Input) *InputValidator {
	return &InputValidator{
		config: config,
		input:  input,
	}
}

func (v *InputValidator) Validate(ctx context.Context) error {
	if v.input == nil {
		return nil
	}
	inputSchema := v.config.Opts.InputSchema
	return schema.NewParamsValidator(v.input, inputSchema, v.config.ID).Validate(ctx)
}

// -----------------------------------------------------------------------------
// ScheduleValidator - Validates workflow schedule configuration
// -----------------------------------------------------------------------------

type ScheduleValidator struct {
	config *Config
}

func NewScheduleValidator(config *Config) *ScheduleValidator {
	return &ScheduleValidator{config: config}
}

func (v *ScheduleValidator) Validate() error {
	// Skip validation if no schedule is configured
	if v.config.Schedule == nil {
		return nil
	}
	// Validate the schedule configuration
	if err := ValidateSchedule(v.config.Schedule); err != nil {
		return fmt.Errorf("schedule validation error: %w", err)
	}
	// Validate schedule input against workflow input schema if present
	if v.config.Opts.InputSchema != nil {
		// Use schedule input if provided, otherwise use an empty map.
		// This ensures validation catches missing required inputs without defaults.
		inputData := v.config.Schedule.Input
		if inputData == nil {
			inputData = make(map[string]any)
		}
		// Apply defaults from schema before validation
		mergedInput, err := v.config.Opts.InputSchema.ApplyDefaults(inputData)
		if err != nil {
			return fmt.Errorf("failed to apply schedule input defaults: %w", err)
		}
		input := core.Input(mergedInput)
		validator := NewInputValidator(v.config, &input)
		// Use background context for schema validation
		if err := validator.Validate(context.Background()); err != nil {
			return fmt.Errorf("schedule input validation error: %w", err)
		}
	}
	return nil
}
