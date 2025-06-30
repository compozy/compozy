package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

type ExecuteTaskInput struct {
	TaskConfig     *task.Config     `json:"task_config"`
	WorkflowState  *workflow.State  `json:"workflow_state"`
	WorkflowConfig *workflow.Config `json:"workflow_config"`
	ProjectConfig  *project.Config  `json:"project_config"`
}

type ExecuteTask struct {
	runtime        *runtime.Manager
	memoryManager  memcore.ManagerInterface
	templateEngine *tplengine.TemplateEngine
}

func NewExecuteTask(
	runtime *runtime.Manager,
	memoryManager memcore.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
) *ExecuteTask {
	return &ExecuteTask{
		runtime:        runtime,
		memoryManager:  memoryManager,
		templateEngine: templateEngine,
	}
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
		result, err = uc.executeAgent(ctx, agentConfig, actionID, input.TaskConfig.With, input)
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
	input *ExecuteTaskInput,
) (*core.Output, error) {
	log := logger.FromContext(ctx)
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

	// Create LLM service options
	var llmOpts []llm.Option

	// Integrate memory resolver if memory manager is available
	hasMemoryDependencies := uc.memoryManager != nil && uc.templateEngine != nil
	hasWorkflowContext := input.WorkflowState != nil

	if hasMemoryDependencies && hasWorkflowContext {
		// Build workflow context for template evaluation
		workflowContext := buildWorkflowContext(
			input.WorkflowState,
			input.WorkflowConfig,
			input.TaskConfig,
			input.ProjectConfig,
		)

		// Create memory resolver for this execution
		memoryResolver := NewMemoryResolver(uc.memoryManager, uc.templateEngine, workflowContext)
		llmOpts = append(llmOpts, llm.WithMemoryProvider(memoryResolver))
	} else if len(agentConfig.GetResolvedMemoryReferences()) > 0 {
		// Log warning if agent has memory configuration but memory manager not available
		log.Warn("Agent has memory configuration but memory manager not available",
			"agent_id", agentConfig.ID,
			"memory_count", len(agentConfig.GetResolvedMemoryReferences()),
		)
	}

	llmService, err := llm.NewService(ctx, uc.runtime, agentConfig, llmOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM service: %w", err)
	}
	// Ensure MCP connections are properly closed when agent execution completes
	defer func() {
		if closeErr := llmService.Close(); closeErr != nil {
			// Log error but don't fail the task
			log.Warn("Failed to close LLM service", "error", closeErr)
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

// buildWorkflowContext creates a context map for template evaluation with workflow data
func buildWorkflowContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	projectConfig *project.Config,
) map[string]any {
	context := make(map[string]any)

	// Add workflow information
	if workflowState != nil {
		workflowData := map[string]any{
			"id":      workflowState.WorkflowID,
			"exec_id": workflowState.WorkflowExecID.String(),
			"status":  workflowState.Status,
		}

		// Add workflow input data under workflow.input
		if workflowState.Input != nil {
			workflowData["input"] = *workflowState.Input
			// Also maintain backward compatibility by putting input at top level
			context["input"] = *workflowState.Input
		}

		// Add workflow outputs if available
		if workflowState.Output != nil {
			workflowData["output"] = *workflowState.Output
			context["output"] = *workflowState.Output
		}

		context["workflow"] = workflowData
	}

	// Add workflow config information
	if workflowConfig != nil {
		context["config"] = map[string]any{
			"id":          workflowConfig.ID,
			"version":     workflowConfig.Version,
			"description": workflowConfig.Description,
		}
	}

	// Add project information - CRITICAL for memory operations
	if projectConfig != nil {
		context["project"] = map[string]any{
			"id":          projectConfig.Name, // Use project name as ID for memory operations
			"name":        projectConfig.Name,
			"version":     projectConfig.Version,
			"description": projectConfig.Description,
		}
	}

	// Add current task information
	if taskConfig != nil {
		context["task"] = map[string]any{
			"id":   taskConfig.ID,
			"type": taskConfig.Type,
		}

		// Add task input if available
		if taskConfig.With != nil {
			context["task_input"] = *taskConfig.With
		}
	}

	// Note: timestamp removed to ensure deterministic memory keys
	// If timestamp is needed, it should be provided by the workflow itself

	return context
}
