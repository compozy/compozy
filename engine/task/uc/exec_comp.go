package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
)

// -----------------------------------------------------------------------------
// ExecComponent
// -----------------------------------------------------------------------------

type ExecComponentInput struct {
	TaskConfig *task.Config
}

type ExecComponentOutput struct {
	Result *core.Output
}

type ExecComponent struct {
	runtime *runtime.Manager
}

func NewExecComponent(runtime *runtime.Manager) *ExecComponent {
	return &ExecComponent{runtime: runtime}
}

func (uc *ExecComponent) Execute(ctx context.Context, input *ExecComponentInput) (*ExecComponentOutput, error) {
	agentConfig := input.TaskConfig.Agent
	toolConfig := input.TaskConfig.Tool
	var result *core.Output
	var err error
	switch {
	case agentConfig != nil:
		actionID := input.TaskConfig.Action
		// TODO: remove this when do automatically selection for action
		if actionID == "" {
			return nil, fmt.Errorf("action ID is required for agent")
		}
		result, err = uc.executeAgent(ctx, agentConfig, actionID)
		if err != nil {
			return nil, fmt.Errorf("failed to execute agent: %w", err)
		}
		return &ExecComponentOutput{
			Result: result,
		}, nil
	case toolConfig != nil:
		result, err = uc.executeTool(ctx, input.TaskConfig, toolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool: %w", err)
		}
		return &ExecComponentOutput{
			Result: result,
		}, nil
	default:
		return nil, fmt.Errorf("no component specified for execution")
	}
}

func (uc *ExecComponent) executeAgent(
	ctx context.Context,
	agentConfig *agent.Config,
	actionID string,
) (*core.Output, error) {
	actionConfig, err := agent.FindActionConfig(agentConfig.Actions, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find action config: %w", err)
	}

	llmService := llm.NewService(uc.runtime, agentConfig, actionConfig)
	result, err := llmService.GenerateContent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	return result, nil
}

func (uc *ExecComponent) executeTool(
	ctx context.Context,
	taskConfig *task.Config,
	toolConfig *tool.Config,
) (*core.Output, error) {
	tool := llm.NewTool(toolConfig, toolConfig.Env, uc.runtime)
	output, err := tool.Call(ctx, taskConfig.With)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}
	return output, nil
}
