package workflow

import (
	"context"
	"fmt"
	"text/template"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/webhook"
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

func (v *Validator) Validate(ctx context.Context) error {
	validator := schema.NewCompositeValidator(
		schema.NewCWDValidator(v.config.CWD, v.config.ID),
		NewTasksValidator(v.config),
		NewAgentsValidator(v.config),
		NewToolsValidator(v.config),
		NewMCPsValidator(v.config),
		NewTriggersValidator(v.config),
		NewOutputsValidator(v.config),
	)
	if err := validator.Validate(); err != nil {
		return err
	}
	// ScheduleValidator needs context, so call it separately
	scheduleValidator := NewScheduleValidator(v.config)
	return scheduleValidator.Validate(ctx)
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
	if len(v.config.Tools) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(v.config.Tools))
	for i := range v.config.Tools {
		tc := &v.config.Tools[i]
		// Validate tool configuration
		if err := tc.Validate(); err != nil {
			return fmt.Errorf("tool validation error: %s", err)
		}
		// Check required ID
		if tc.ID == "" {
			return fmt.Errorf("tool[%d] missing required ID field", i)
		}
		// Detect duplicates
		if _, ok := seen[tc.ID]; ok {
			return fmt.Errorf("duplicate tool ID '%s' found in workflow tools", tc.ID)
		}
		seen[tc.ID] = struct{}{}
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
	signalNames := map[string]struct{}{}
	for i := range v.config.Triggers {
		t := &v.config.Triggers[i]
		switch t.Type {
		case TriggerTypeSignal:
			if err := validateSignalTrigger(t, signalNames, v.config.ID, i); err != nil {
				return err
			}
		case TriggerTypeWebhook:
			if t.Webhook == nil {
				return fmt.Errorf(
					"workflow '%s' trigger[%d]: webhook config is required for webhook triggers",
					v.config.ID,
					i,
				)
			}
			if t.Name != "" {
				return fmt.Errorf("workflow '%s' trigger[%d]: name is not allowed for webhook triggers", v.config.ID, i)
			}
			if err := webhook.ValidateTrigger(t.Webhook); err != nil {
				return fmt.Errorf("workflow '%s' trigger[%d]: invalid webhook trigger: %w", v.config.ID, i, err)
			}
		default:
			return fmt.Errorf("workflow '%s' trigger[%d]: unsupported trigger type: %s", v.config.ID, i, t.Type)
		}
	}
	return nil
}

func validateSignalTrigger(t *Trigger, seen map[string]struct{}, workflowID string, triggerIndex int) error {
	if t.Name == "" {
		return fmt.Errorf("workflow '%s' trigger[%d]: trigger name is required", workflowID, triggerIndex)
	}
	if t.Webhook != nil {
		return fmt.Errorf(
			"workflow '%s' trigger[%d] '%s': webhook config is not allowed for signal triggers",
			workflowID,
			triggerIndex,
			t.Name,
		)
	}
	if _, dup := seen[t.Name]; dup {
		return fmt.Errorf("workflow '%s' trigger[%d] '%s': duplicate trigger name", workflowID, triggerIndex, t.Name)
	}
	seen[t.Name] = struct{}{}
	if t.Schema != nil {
		if _, err := t.Schema.Compile(); err != nil {
			return fmt.Errorf(
				"workflow '%s' trigger[%d] '%s': invalid trigger schema: %w",
				workflowID,
				triggerIndex,
				t.Name,
				err,
			)
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

func (v *ScheduleValidator) Validate(ctx context.Context) error {
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
		if err := validator.Validate(ctx); err != nil {
			return fmt.Errorf("schedule input validation error: %w", err)
		}
	}
	return nil
}
