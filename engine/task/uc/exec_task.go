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
	"github.com/compozy/compozy/pkg/logger"
)

type ExecuteTaskInput struct {
	TaskConfig *task.Config `json:"task_config"`
}

type ExecuteTask struct {
	runtime *runtime.Manager
}

func NewExecuteTask(runtime *runtime.Manager) *ExecuteTask {
	return &ExecuteTask{runtime: runtime}
}

func (uc *ExecuteTask) Execute(ctx context.Context, input *ExecuteTaskInput) (*core.Output, error) {
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
		result, err = uc.executeAgent(ctx, agentConfig, actionID, input.TaskConfig.With)
		if err != nil {
			return nil, fmt.Errorf("failed to execute agent: %w", err)
		}
		return result, nil
	case toolConfig != nil:
		result, err = uc.executeTool(ctx, input.TaskConfig, toolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool: %w", err)
		}
		return result, nil
	}
	// This should be unreachable for valid basic tasks due to load-time validation
	return nil, fmt.Errorf(
		"unreachable: task (ID: %s, Type: %s) has no executable component (agent/tool); validation may be misconfigured",
		input.TaskConfig.ID,
		input.TaskConfig.Type,
	)
}

func (uc *ExecuteTask) executeAgent(
	ctx context.Context,
	agentConfig *agent.Config,
	actionID string,
	taskWith *core.Input,
) (*core.Output, error) {
	actionConfig, err := agent.FindActionConfig(agentConfig.Actions, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find action config: %w", err)
	}

	// Create a deep copy of the action config with task's runtime input data
	runtimeActionConfig, err := actionConfig.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to deep copy action config: %w", err)
	}
	if taskWith != nil {
		inputCopy, err := taskWith.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone task with: %w", err)
		}
		runtimeActionConfig.With = inputCopy
	}

	llmService, err := llm.NewService(uc.runtime, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM service: %w", err)
	}
	// Ensure MCP connections are properly closed when agent execution completes
	defer func() {
		if closeErr := llmService.Close(); closeErr != nil {
			// Log error but don't fail the task
			logger.Warn("failed to close LLM service", "error", closeErr)
		}
	}()

	result, err := llmService.GenerateContent(ctx, agentConfig, runtimeActionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	return result, nil
}

func (uc *ExecuteTask) executeTool(
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
