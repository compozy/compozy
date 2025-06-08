package llm

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
)

type Tool struct {
	config  *tool.Config
	runtime *runtime.Manager
	env     *core.EnvMap
}

func NewTool(config *tool.Config, env *core.EnvMap, runtime *runtime.Manager) *Tool {
	return &Tool{
		config:  config,
		env:     env,
		runtime: runtime,
	}
}

func (t *Tool) Name() string {
	return t.config.ID
}

func (t *Tool) Description() string {
	return t.config.Description
}

func (t *Tool) Call(ctx context.Context, input *core.Input) (*core.Output, error) {
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
	if output != nil {
		if resultData, ok := (*output)["result"].(map[string]any); ok {
			newOutput := core.Output(resultData)
			return &newOutput, nil
		}
	}
	return output, nil
}

// validateInput parses the input JSON and validates it against the input schema
func (t *Tool) validateInput(ctx context.Context, input *core.Input) (*core.Input, error) {
	if err := t.config.ValidateInput(ctx, input); err != nil {
		return nil, fmt.Errorf("input schema validation failed: %w", err)
	}
	return input, nil
}

func (t *Tool) validateOutput(ctx context.Context, output *core.Output) error {
	if t.config.OutputSchema == nil {
		return nil
	}
	return t.config.ValidateOutput(ctx, output)
}

// executeTool executes the tool with the runtime manager
func (t *Tool) executeTool(ctx context.Context, input *core.Input) (*core.Output, error) {
	if t.runtime == nil {
		return nil, fmt.Errorf("runtime manager is nil")
	}
	
	toolExecID := core.MustNewID()
	env := core.EnvMap{}
	if t.env != nil {
		env = *t.env
	}
	return t.runtime.ExecuteTool(ctx, t.config.ID, toolExecID, input, env)
}
