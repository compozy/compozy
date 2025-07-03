package llm

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
)

type InternalTool struct {
	config  *tool.Config
	runtime runtime.Runtime
	env     *core.EnvMap
}

func NewTool(config *tool.Config, env *core.EnvMap, runtime runtime.Runtime) *InternalTool {
	return &InternalTool{
		config:  config,
		env:     env,
		runtime: runtime,
	}
}

func (t *InternalTool) Name() string {
	return t.config.ID
}

func (t *InternalTool) Description() string {
	return t.config.Description
}

func (t *InternalTool) Call(ctx context.Context, input *core.Input) (*core.Output, error) {
	inputMap, err := t.validateInput(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}
	output, err := t.executeTool(ctx, inputMap)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	err = t.validateOutput(ctx, output)
	if err != nil {
		return nil, fmt.Errorf("output processing failed: %w", err)
	}
	// Return output directly - runtime manager handles all parsing and normalization
	return output, nil
}

// validateInput parses the input JSON and validates it against the input schema
func (t *InternalTool) validateInput(ctx context.Context, input *core.Input) (*core.Input, error) {
	if err := t.config.ValidateInput(ctx, input); err != nil {
		return nil, fmt.Errorf("input schema validation failed: %w", err)
	}
	return input, nil
}

func (t *InternalTool) validateOutput(ctx context.Context, output *core.Output) error {
	if t.config.OutputSchema == nil {
		return nil
	}
	return t.config.ValidateOutput(ctx, output)
}

// executeTool executes the tool with the runtime manager using tool-specific timeout
func (t *InternalTool) executeTool(ctx context.Context, input *core.Input) (*core.Output, error) {
	toolExecID := core.MustNewID()
	env := core.EnvMap{}
	if t.env != nil {
		env = *t.env
	}
	globalTimeout := t.runtime.GetGlobalTimeout()
	toolTimeout, err := t.config.GetTimeout(globalTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool timeout: %w", err)
	}
	return t.runtime.ExecuteToolWithTimeout(ctx, t.config.ID, toolExecID, input, env, toolTimeout)
}
