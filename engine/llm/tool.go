package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
)

type InternalTool struct {
	config  *tool.Config
	runtime runtime.Runtime
	env     *core.EnvMap
	builtin *builtin.BuiltinDefinition
}

func NewTool(
	config *tool.Config,
	env *core.EnvMap,
	runtime runtime.Runtime,
	toolEnv toolenv.Environment,
) *InternalTool {
	internal := &InternalTool{
		config:  config,
		env:     env,
		runtime: runtime,
	}
	if config != nil && strings.HasPrefix(config.ID, "cp__") {
		if toolEnv != nil {
			if def, ok := native.DefinitionByIDWithEnvironment(toolEnv, config.ID); ok {
				defCopy := def
				internal.builtin = &defCopy
			}
		} else if def, ok := native.DefinitionByID(config.ID); ok {
			defCopy := def
			internal.builtin = &defCopy
		}
	}
	return internal
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
	config := t.config.GetConfig()
	output, err := t.executeTool(ctx, inputMap, config)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	err = t.validateOutput(ctx, output)
	if err != nil {
		return nil, fmt.Errorf("output processing failed: %w", err)
	}
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
func (t *InternalTool) executeTool(ctx context.Context, input *core.Input, config *core.Input) (*core.Output, error) {
	if t.builtin != nil {
		payload := map[string]any{}
		if input != nil {
			payload = input.AsMap()
		}
		outputMap, err := t.builtin.Handler(ctx, payload)
		if err != nil {
			return nil, err
		}
		if outputMap == nil {
			outputMap = core.Output{}
		}
		return &outputMap, nil
	}
	toolExecID := core.MustNewID()
	env := core.EnvMap{}
	if t.env != nil {
		env = core.CloneMap(*t.env)
	}
	globalTimeout := t.runtime.GetGlobalTimeout()
	toolTimeout, err := t.config.GetTimeout(ctx, globalTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool timeout: %w", err)
	}
	return t.runtime.ExecuteToolWithTimeout(ctx, t.config.ID, toolExecID, input, config, env, toolTimeout)
}
