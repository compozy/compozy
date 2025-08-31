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
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
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
	runtime        runtime.Runtime
	workflowRepo   workflow.Repository
	memoryManager  memcore.ManagerInterface
	templateEngine *tplengine.TemplateEngine
	appConfig      *config.Config
}

// NewExecuteTask creates a new ExecuteTask configured with the provided runtime, workflow
// repository, memory manager, template engine, and application configuration. The returned
// ExecuteTask coordinates task execution (agent or tool) using those injected dependencies.
func NewExecuteTask(
	runtime runtime.Runtime,
	workflowRepo workflow.Repository,
	memoryManager memcore.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
	appConfig *config.Config,
) *ExecuteTask {
	return &ExecuteTask{
		runtime:        runtime,
		workflowRepo:   workflowRepo,
		memoryManager:  memoryManager,
		templateEngine: templateEngine,
		appConfig:      appConfig,
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
		promptText := input.TaskConfig.Prompt

		// Trust load-time validation - both action and prompt can be provided together for enhanced context
		// At least one of them is guaranteed to be present by validators.go

		result, err = uc.executeAgent(ctx, agentConfig, actionID, promptText, input.TaskConfig.With, input)
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

// reparseAgentConfig re-parses agent configuration templates at runtime with full workflow context
func (uc *ExecuteTask) reparseAgentConfig(
	ctx context.Context,
	agentConfig *agent.Config,
	input *ExecuteTaskInput,
	actionID string,
) error {
	if input.WorkflowState == nil {
		return nil
	}

	// Build normalization context for runtime re-parsing
	normCtx := &shared.NormalizationContext{
		WorkflowState:  input.WorkflowState,
		WorkflowConfig: input.WorkflowConfig,
		TaskConfig:     input.TaskConfig,
		Variables:      make(map[string]any),
		CurrentInput:   input.TaskConfig.With, // Include task's current input for collection variables
	}

	// Build template context with all workflow data including tasks outputs
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return fmt.Errorf("failed to create context builder: %w", err)
	}

	// Build full context with tasks data
	fullCtx := contextBuilder.BuildContext(input.WorkflowState, input.WorkflowConfig, input.TaskConfig)
	normCtx.Variables = fullCtx.Variables

	// Ensure task's current input (containing collection variables) is added to variables
	if input.TaskConfig.With != nil {
		vb := shared.NewVariableBuilder()
		vb.AddCurrentInputToVariables(normCtx.Variables, input.TaskConfig.With)
	}

	// Create agent normalizer for runtime re-parsing
	// Initialize EnvMerger to avoid nil-pointer dereference in AgentNormalizer
	envMerger := task2core.NewEnvMerger()
	agentNormalizer := task2core.NewAgentNormalizer(envMerger)

	// Re-parse the agent configuration with runtime context
	// Pass actionID to only reparse the specific action being executed
	if err := agentNormalizer.ReparseInput(agentConfig, normCtx, actionID); err != nil {
		return fmt.Errorf("runtime template parse failed: %w", err)
	}

	log := logger.FromContext(ctx)
	log.Debug("Successfully re-parsed agent configuration at runtime",
		"agent_id", agentConfig.ID,
		"task_id", input.TaskConfig.ID)

	return nil
}

func (uc *ExecuteTask) setupMemoryResolver(
	input *ExecuteTaskInput,
	agentConfig *agent.Config,
) ([]llm.Option, bool) {
	var llmOpts []llm.Option
	hasMemoryConfig := false

	// Add app config if available
	if uc.appConfig != nil {
		llmOpts = append(llmOpts, llm.WithAppConfig(uc.appConfig))
	}

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
	} else if len(agentConfig.Memory) > 0 {
		hasMemoryConfig = true
	}

	return llmOpts, hasMemoryConfig
}

// refreshWorkflowState ensures we operate on the latest workflow state so template parsing
// can see freshly produced task outputs.
func (uc *ExecuteTask) refreshWorkflowState(ctx context.Context, input *ExecuteTaskInput) {
	// Ensure we have the minimal data we need
	if input == nil || input.WorkflowState == nil {
		return
	}

	execID := input.WorkflowState.WorkflowExecID
	if execID == "" {
		return
	}

	if uc.workflowRepo == nil {
		// If no workflow repo available (e.g., in ExecuteBasic), skip refresh
		return
	}

	freshState, err := uc.workflowRepo.GetState(ctx, execID)
	if err != nil {
		// Non-fatal: log and keep the old snapshot
		logger.FromContext(ctx).Warn("failed to refresh workflow state; continuing with stale snapshot",
			"exec_id", execID.String(),
			"error", err)
		return
	}

	input.WorkflowState = freshState
}

func (uc *ExecuteTask) executeAgent(
	ctx context.Context,
	agentConfig *agent.Config,
	actionID string,
	promptText string,
	taskWith *core.Input,
	input *ExecuteTaskInput,
) (*core.Output, error) {
	log := logger.FromContext(ctx)

	// Ensure we operate on the latest workflow state so template parsing sees
	// freshly produced task outputs (e.g., read_content inside collections).
	uc.refreshWorkflowState(ctx, input)

	// Re-parse agent configuration templates at runtime with full workflow context
	// This MUST happen BEFORE cloning action config so the clone gets updated templates
	// This is critical for collection subtasks where .tasks.* references need actual data
	log.Debug("About to re-parse agent configuration",
		"agent_id", agentConfig.ID,
		"action_id", actionID,
		"task_id", input.TaskConfig.ID)
	if err := uc.reparseAgentConfig(ctx, agentConfig, input, actionID); err != nil {
		log.Warn("Failed to re-parse agent configuration at runtime",
			"agent_id", agentConfig.ID,
			"error", err)
	} else {
		log.Debug("Successfully completed re-parsing of agent configuration",
			"agent_id", agentConfig.ID,
			"action_id", actionID)
	}

	// Setup memory resolver and LLM options
	llmOpts, hasMemoryConfig := uc.setupMemoryResolver(input, agentConfig)
	if hasMemoryConfig {
		log.Warn("Agent has memory configuration but memory manager not available",
			"agent_id", agentConfig.ID,
			"memory_count", len(agentConfig.Memory),
			"action", "Memory features will be disabled for this execution")
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

	// Resolve templates in promptText if present and context available
	resolvedPrompt := promptText
	if promptText != "" && uc.templateEngine != nil && input.WorkflowState != nil {
		workflowContext := buildWorkflowContext(
			input.WorkflowState,
			input.WorkflowConfig,
			input.TaskConfig,
			input.ProjectConfig,
		)
		if rendered, rerr := uc.templateEngine.RenderString(promptText, workflowContext); rerr == nil {
			resolvedPrompt = rendered
		} else {
			log.Warn("Failed to resolve templates in prompt; using raw value",
				"error", rerr)
		}
	}

	result, err := llmService.GenerateContent(ctx, agentConfig, taskWith, actionID, resolvedPrompt)
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
			workflowData["input"] = dereferenceInput(workflowState.Input)
			// Also maintain backward compatibility by putting input at top level
			context["input"] = dereferenceInput(workflowState.Input)
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

// dereferenceInput safely dereferences the workflow input pointer for template resolution
func dereferenceInput(input *core.Input) any {
	if input == nil {
		return nil
	}
	// Dereference the pointer to expose the underlying map
	// This allows templates to access nested fields like .workflow.input.user_id
	return *input
}
