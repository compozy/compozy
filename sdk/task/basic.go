package task

import (
	"context"
	"fmt"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const defaultOutputAlias = "result"

// BasicBuilder creates engine basic task configurations with fluent helpers while collecting validation errors.
type BasicBuilder struct {
	config *enginetask.Config
	errors []error
}

// NewBasic constructs a basic task builder initialized with the provided identifier.
func NewBasic(id string) *BasicBuilder {
	trimmed := strings.TrimSpace(id)
	return &BasicBuilder{
		config: &enginetask.Config{
			BaseConfig: enginetask.BaseConfig{
				Resource: string(core.ConfigTask),
				ID:       trimmed,
				Type:     enginetask.TaskTypeBasic,
			},
		},
		errors: make([]error, 0),
	}
}

// WithAgent configures the task to execute using the referenced agent identifier.
func (b *BasicBuilder) WithAgent(agentID string) *BasicBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("agent id cannot be empty"))
		b.config.Agent = nil
		return b
	}
	b.config.Agent = &engineagent.Config{
		Resource: string(core.ConfigAgent),
		ID:       trimmed,
	}
	return b
}

// WithAction assigns the action identifier executed when the agent runs.
func (b *BasicBuilder) WithAction(actionID string) *BasicBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(actionID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("action id cannot be empty"))
		return b
	}
	b.config.Action = trimmed
	return b
}

// WithTool configures the task to execute using the referenced tool identifier.
func (b *BasicBuilder) WithTool(toolID string) *BasicBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(toolID)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("tool id cannot be empty"))
		b.config.Tool = nil
		return b
	}
	b.config.Tool = &enginetool.Config{
		Resource: string(core.ConfigTool),
		ID:       trimmed,
	}
	return b
}

// WithInput registers template-driven input parameters available during task execution.
func (b *BasicBuilder) WithInput(input map[string]string) *BasicBuilder {
	if b == nil {
		return nil
	}
	if input == nil {
		b.config.With = nil
		return b
	}
	values := make(map[string]any, len(input))
	for rawKey, value := range input {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			b.errors = append(b.errors, fmt.Errorf("input key cannot be empty"))
			continue
		}
		values[key] = value
	}
	mapped := core.Input(values)
	b.config.With = &mapped
	return b
}

// WithOutput adds an output mapping using either "alias=expression" syntax or defaults to alias "result".
func (b *BasicBuilder) WithOutput(output string) *BasicBuilder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("output mapping cannot be empty"))
		return b
	}
	key := defaultOutputAlias
	expression := trimmed
	if idx := strings.Index(trimmed, "="); idx >= 0 {
		key = strings.TrimSpace(trimmed[:idx])
		expression = strings.TrimSpace(trimmed[idx+1:])
	} else if idx := strings.Index(trimmed, ":"); idx >= 0 {
		key = strings.TrimSpace(trimmed[:idx])
		expression = strings.TrimSpace(trimmed[idx+1:])
	}
	if key == "" {
		b.errors = append(b.errors, fmt.Errorf("output mapping key cannot be empty"))
		return b
	}
	if expression == "" {
		b.errors = append(b.errors, fmt.Errorf("output mapping expression cannot be empty"))
		return b
	}
	base := make(map[string]any)
	if b.config.Outputs != nil {
		base = core.CloneMap(map[string]any(*b.config.Outputs))
	}
	base[key] = expression
	mapped := core.Input(base)
	b.config.Outputs = &mapped
	return b
}

// WithCondition attaches a CEL expression that gates task execution.
func (b *BasicBuilder) WithCondition(condition string) *BasicBuilder {
	if b == nil {
		return nil
	}
	b.config.Condition = strings.TrimSpace(condition)
	return b
}

// WithFinal marks whether this task terminates workflow execution when completed.
func (b *BasicBuilder) WithFinal(isFinal bool) *BasicBuilder {
	if b == nil {
		return nil
	}
	b.config.Final = isFinal
	return b
}

// Build validates the accumulated configuration and returns a fully-populated engine task config.
func (b *BasicBuilder) Build(ctx context.Context) (*enginetask.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("basic builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	hasAgent := b.config.Agent != nil
	hasTool := b.config.Tool != nil
	log.Debug("building basic task configuration", "task", b.config.ID, "hasAgent", hasAgent, "hasTool", hasTool)
	collected := make([]error, 0, len(b.errors)+3)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateID(ctx))
	b.validateCondition()
	collected = append(collected, b.validateExecutionMode(ctx))
	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	b.config.Type = enginetask.TaskTypeBasic
	b.config.Resource = string(core.ConfigTask)
	cloned, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone basic task config: %w", err)
	}
	return cloned, nil
}

func (b *BasicBuilder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("task id is invalid: %w", err)
	}
	return nil
}

func (b *BasicBuilder) validateCondition() {
	b.config.Condition = strings.TrimSpace(b.config.Condition)
}

func (b *BasicBuilder) validateExecutionMode(ctx context.Context) error {
	hasAgent := b.config.Agent != nil
	hasTool := b.config.Tool != nil
	if hasAgent && hasTool {
		return fmt.Errorf("basic tasks cannot reference both an agent and a tool")
	}
	if !hasAgent && !hasTool {
		return fmt.Errorf("basic tasks must reference either an agent or a tool")
	}
	if hasAgent {
		b.config.Agent.ID = strings.TrimSpace(b.config.Agent.ID)
		if err := validate.ID(ctx, b.config.Agent.ID); err != nil {
			return fmt.Errorf("agent id is invalid: %w", err)
		}
		b.config.Agent.Resource = string(core.ConfigAgent)
		b.config.Action = strings.TrimSpace(b.config.Action)
		if b.config.Action == "" {
			return fmt.Errorf("basic tasks using agents must specify an action")
		}
	}
	if hasTool {
		b.config.Tool.ID = strings.TrimSpace(b.config.Tool.ID)
		if err := validate.ID(ctx, b.config.Tool.ID); err != nil {
			return fmt.Errorf("tool id is invalid: %w", err)
		}
		b.config.Tool.Resource = string(core.ConfigTool)
		b.config.Action = strings.TrimSpace(b.config.Action)
		if b.config.Action != "" {
			return fmt.Errorf("basic tasks using tools cannot specify an action")
		}
	}
	return nil
}
