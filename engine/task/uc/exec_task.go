package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
)

type ExecuteTaskInput struct {
	TaskConfig   *task.Config `json:"task_config"`
	WorkflowMCPs []mcp.Config `json:"workflow_mcps,omitempty"`
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
		result, err = uc.executeAgent(ctx, agentConfig, actionID, input.WorkflowMCPs)
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
	default:
		return nil, fmt.Errorf("no component specified for execution")
	}
}

func (uc *ExecuteTask) executeAgent(
	ctx context.Context,
	agentConfig *agent.Config,
	actionID string,
	workflowMCPs []mcp.Config,
) (*core.Output, error) {
	actionConfig, err := agent.FindActionConfig(agentConfig.Actions, actionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find action config: %w", err)
	}

	llmService, err := llm.NewService(uc.runtime, agentConfig, actionConfig, workflowMCPs)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM service: %w", err)
	}
	// Ensure MCP connections are properly closed when agent execution completes
	defer func() {
		if closeErr := llmService.Close(); closeErr != nil {
			// Log error but don't fail the task
			fmt.Printf("Warning: failed to close LLM service: %v\n", closeErr)
		}
	}()

	result, err := llmService.GenerateContent(ctx)
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
