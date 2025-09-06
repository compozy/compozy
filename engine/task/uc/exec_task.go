package uc

import (
	"context"
	"fmt"
	"strings"

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
	"github.com/compozy/compozy/engine/tool/resolver"
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
	toolResolver   resolver.ToolResolver
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
	toolResolver resolver.ToolResolver,
) *ExecuteTask {
	return &ExecuteTask{
		runtime:        runtime,
		workflowRepo:   workflowRepo,
		memoryManager:  memoryManager,
		templateEngine: templateEngine,
		appConfig:      appConfig,
		toolResolver: func() resolver.ToolResolver {
			if toolResolver != nil {
				return toolResolver
			}
			return resolver.NewHierarchicalResolver()
		}(),
	}
}

// resolveModelConfig implements the model resolution fallback chain:
// 1. Task-specific ModelConfig (if set)
// 2. Agent-specific Config (if agent exists and has config)
// 3. Project default model (if configured)
func (uc *ExecuteTask) resolveModelConfig(input *ExecuteTaskInput) error {
	if input == nil || input.TaskConfig == nil {
		return fmt.Errorf("invalid input: missing task configuration")
	}
	// If task already has a model config, nothing to do
	if input.TaskConfig.ModelConfig.Provider != "" {
		return nil
	}
	// Check if agent has a model config
	if input.TaskConfig.Agent != nil && input.TaskConfig.Agent.Config.Provider != "" {
		// Agent has config, will be used during agent execution
		return nil
	}
	// Try to use project default model
	if input.ProjectConfig != nil {
		defaultModel := input.ProjectConfig.GetDefaultModel()
		if defaultModel != nil {
			// Set the default model as the task's model config for direct LLM tasks
			input.TaskConfig.ModelConfig = *defaultModel
			return nil
		}
	}
	// For agent tasks, the agent might have its own config, so we don't error here
	// For direct LLM tasks, we need a model config
	if input.TaskConfig.Agent == nil && input.TaskConfig.Tool == nil && input.TaskConfig.Prompt != "" {
		return fmt.Errorf("no model configuration available: task, agent, and project have no model specified")
	}
	return nil
}

// normalizeProviderConfigWithEnv resolves template expressions inside a ProviderConfig
// (e.g., api_key: "{{ .env.OPENAI_API_KEY }}") using the available environment context.
// It merges environment sources using task2core.EnvMerger.MergeWorkflowToTask:
// - Workflow config env (preferred)
// - Task-level env (merged)
// Ensure project-level env is included by the merger if required by spec.
// The function updates cfg in-place with resolved values.
func (uc *ExecuteTask) normalizeProviderConfigWithEnv(cfg *core.ProviderConfig, input *ExecuteTaskInput) error {
	if cfg == nil {
		return nil
	}
	// Build the standard normalization context using project/workflow/task rules
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return fmt.Errorf("failed to create context builder: %w", err)
	}
	normCtx := contextBuilder.BuildContext(input.WorkflowState, input.WorkflowConfig, input.TaskConfig)

	// Merge env following project standards (workflow -> task)
	// For agent cases, the agent-level env is merged by AgentNormalizer already
	envMerger := task2core.NewEnvMerger()
	merged := envMerger.MergeWorkflowToTask(input.WorkflowConfig, input.TaskConfig)
	if merged != nil {
		// Override top-level env in the template context with merged values
		// to follow standard recursive merging semantics
		if normCtx.Variables == nil {
			normCtx.Variables = make(map[string]any)
		}
		normCtx.Variables["env"] = merged
	}

	// Use existing template engine when available to keep behavior consistent
	engine := uc.templateEngine
	if engine == nil {
		engine = tplengine.NewEngine(tplengine.FormatJSON)
	}

	// Convert config to map, parse templates with the built context, and write back
	cfgMap, err := cfg.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert provider config to map: %w", err)
	}
	parsed, err := engine.ParseAny(
		cfgMap,
		normCtx.BuildTemplateContext(),
	)
	if err != nil {
		return fmt.Errorf("failed to render provider config templates: %w", err)
	}
	if err := cfg.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update provider config from parsed map: %w", err)
	}
	return nil
}

func (uc *ExecuteTask) Execute(ctx context.Context, input *ExecuteTaskInput) (*core.Output, error) {
	// Resolve model configuration with fallback chain
	if err := uc.resolveModelConfig(input); err != nil {
		return nil, fmt.Errorf("failed to resolve model configuration: %w", err)
	}
	agentConfig := input.TaskConfig.Agent
	toolConfig := input.TaskConfig.Tool
	hasDirectLLM := input.TaskConfig.ModelConfig.Provider != "" &&
		input.TaskConfig.ModelConfig.Model != "" &&
		input.TaskConfig.Prompt != ""

	var result *core.Output
	var err error
	switch {
	case agentConfig != nil:
		actionID := input.TaskConfig.Action
		promptText := input.TaskConfig.Prompt

		// Defensive guard: ensure at least one of action or prompt is provided
		if actionID == "" && promptText == "" {
			return nil, fmt.Errorf("agent execution requires action or prompt")
		}

		result, err = uc.executeAgent(ctx, agentConfig, actionID, promptText, input.TaskConfig.With, input)
		if err != nil {
			return nil, fmt.Errorf("failed to execute agent: %w", err)
		}
		return result, nil
	case toolConfig != nil:
		result, err = uc.executeTool(ctx, input, toolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to execute tool: %w", err)
		}
		return result, nil
	case hasDirectLLM:
		// Execute direct LLM task
		result, err = uc.executeDirectLLM(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to execute direct LLM task: %w", err)
		}
		return result, nil
	}
	// This should be unreachable for valid basic tasks due to load-time validation
	return nil, fmt.Errorf(
		"unreachable: task (ID: %s, Type: %s) has no executable component "+
			"(agent/tool/direct LLM); validation may be misconfigured",
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
	if execID.IsZero() {
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
	// Prepare agent configuration
	if err := uc.prepareAgentConfig(ctx, agentConfig, input, actionID); err != nil {
		return nil, err
	}
	if agentConfig.Config.Provider == "" {
		log.Warn("No model provider configured for agent; execution may fail",
			"agent_id", agentConfig.ID,
			"task_id", input.TaskConfig.ID)
	}
	// Create LLM service with resolved tools and memory
	llmService, err := uc.createLLMService(ctx, agentConfig, input)
	if err != nil {
		return nil, err
	}
	// Ensure MCP connections are properly closed when agent execution completes
	defer func() {
		if closeErr := llmService.Close(); closeErr != nil {
			// Log error but don't fail the task
			log.Warn("Failed to close LLM service", "error", closeErr)
		}
	}()
	// Resolve templates in promptText if present and context available
	resolvedPrompt := uc.resolvePromptTemplates(ctx, promptText, agentConfig, input)
	result, err := llmService.GenerateContent(ctx, agentConfig, taskWith, actionID, resolvedPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}
	return result, nil
}

// prepareAgentConfig handles agent configuration preparation including default model and template normalization
func (uc *ExecuteTask) prepareAgentConfig(
	ctx context.Context,
	agentConfig *agent.Config,
	input *ExecuteTaskInput,
	actionID string,
) error {
	log := logger.FromContext(ctx)
	// Apply default model to agent if it doesn't have one
	if agentConfig.Config.Provider == "" && input.ProjectConfig != nil {
		defaultModel := input.ProjectConfig.GetDefaultModel()
		if defaultModel != nil {
			agentConfig.Config = *defaultModel
		}
	}
	// If provider is set but model is empty, try to fill from project default (same provider)
	if agentConfig.Config.Provider != "" && agentConfig.Config.Model == "" && input.ProjectConfig != nil {
		if dm := input.ProjectConfig.GetDefaultModel(); dm != nil && dm.Provider == agentConfig.Config.Provider {
			agentConfig.Config.Model = dm.Model
		}
	}
	// Ensure provider config templates (like API keys) are normalized with env
	if err := uc.normalizeProviderConfigWithEnv(&agentConfig.Config, input); err != nil {
		return fmt.Errorf("failed to normalize provider config: %w", err)
	}
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
	return nil
}

// createLLMService creates the LLM service with resolved tools and memory configuration
func (uc *ExecuteTask) createLLMService(
	ctx context.Context,
	agentConfig *agent.Config,
	input *ExecuteTaskInput,
) (*llm.Service, error) {
	log := logger.FromContext(ctx)
	// Setup memory resolver and LLM options
	llmOpts, hasMemoryConfig := uc.setupMemoryResolver(input, agentConfig)
	if hasMemoryConfig {
		log.Warn("Agent memory configured but runtime dependencies unavailable; disabling memory for this run",
			"agent_id", agentConfig.ID,
			"memory_count", len(agentConfig.Memory),
			"has_manager", uc.memoryManager != nil,
			"has_template_engine", uc.templateEngine != nil,
			"has_workflow_context", input.WorkflowState != nil)
	}
	// Resolve tools using hierarchical inheritance
	resolvedTools, err := uc.toolResolver.ResolveTools(input.ProjectConfig, input.WorkflowConfig, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve tools: %w", err)
	}
	// Merge environment inheritance for tools: workflow -> task -> agent -> tool
	if len(resolvedTools) > 0 {
		envMerger := task2core.NewEnvMerger()
		// Base: workflow + task
		baseEnv := envMerger.MergeWorkflowToTask(input.WorkflowConfig, input.TaskConfig)
		// Add agent env on top of base (if any)
		baseWithAgent := envMerger.MergeForComponent(baseEnv, agentConfig.Env)
		// For every resolved tool, apply component-level overrides
		for i := range resolvedTools {
			merged := envMerger.MergeForComponent(baseWithAgent, resolvedTools[i].Env)
			resolvedTools[i].Env = merged
		}
		// Add resolved tools (with merged env) to LLM options
		llmOpts = append(llmOpts, llm.WithResolvedTools(resolvedTools))
	}

	// Build MCP allowlist from agent/workflow declarations (IDs)
	if ids := uc.allowedMCPIDs(agentConfig, input); len(ids) > 0 {
		llmOpts = append(llmOpts, llm.WithAllowedMCPNames(ids))
	}
	// MCP registration is handled by server startup (engine/mcp.SetupForWorkflows)
	// We no longer register workflow-level MCPs from the exec task.
	llmService, err := llm.NewService(ctx, uc.runtime, agentConfig, llmOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM service: %w", err)
	}
	return llmService, nil
}

// resolvePromptTemplates handles template resolution in the prompt text
func (uc *ExecuteTask) resolvePromptTemplates(
	ctx context.Context,
	promptText string,
	agentConfig *agent.Config,
	input *ExecuteTaskInput,
) string {
	log := logger.FromContext(ctx)
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
				"agent_id", agentConfig.ID,
				"task_id", input.TaskConfig.ID,
				"reason", "template_render_failed")
		}
	}
	return resolvedPrompt
}

// executeDirectLLM executes a task with direct LLM configuration (no agent)
func (uc *ExecuteTask) executeDirectLLM(ctx context.Context, input *ExecuteTaskInput) (*core.Output, error) {
	// Preflight validation for clearer failures
	if input == nil || input.TaskConfig == nil {
		return nil, fmt.Errorf("invalid task input: missing task configuration")
	}
	if input.TaskConfig.ModelConfig.Provider == "" || input.TaskConfig.ModelConfig.Model == "" {
		return nil, fmt.Errorf("direct LLM requires provider and model (task_id=%s)", input.TaskConfig.ID)
	}
	if input.TaskConfig.Prompt == "" {
		return nil, fmt.Errorf("direct LLM requires a non-empty prompt (task_id=%s)", input.TaskConfig.ID)
	}

	// Build a synthetic agent config from the task's LLM properties
	syntheticAgent := &agent.Config{
		ID:            fmt.Sprintf("task-%s-llm", input.TaskConfig.ID),
		Instructions:  "Direct LLM task execution - follow the task prompt",
		Config:        input.TaskConfig.ModelConfig,
		LLMProperties: input.TaskConfig.LLMProperties,
	}
	logger.FromContext(ctx).Debug("Executing direct LLM task",
		"task_id", input.TaskConfig.ID,
		"provider", input.TaskConfig.ModelConfig.Provider,
		"model", input.TaskConfig.ModelConfig.Model)
	// Normalize provider config for direct LLM before execution
	if err := uc.normalizeProviderConfigWithEnv(&syntheticAgent.Config, input); err != nil {
		return nil, fmt.Errorf("failed to normalize provider config for direct LLM: %w", err)
	}
	promptText := input.TaskConfig.Prompt
	// We don't need an action ID for direct LLM execution since we're using the task prompt
	return uc.executeAgent(ctx, syntheticAgent, "", promptText, input.TaskConfig.With, input)
}

func (uc *ExecuteTask) executeTool(
	ctx context.Context,
	input *ExecuteTaskInput,
	toolConfig *tool.Config,
) (*core.Output, error) {
	// Ensure direct tool execution receives merged environment: workflow -> task -> tool
	envMerger := task2core.NewEnvMerger()
	baseEnv := envMerger.MergeWorkflowToTask(input.WorkflowConfig, input.TaskConfig)
	mergedEnv := envMerger.MergeForComponent(baseEnv, toolConfig.Env)
	tool := llm.NewTool(toolConfig, mergedEnv, uc.runtime)
	output, err := tool.Call(ctx, input.TaskConfig.With)
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

	return context
}

// dereferenceInput safely dereferences the workflow input pointer for template resolution
func dereferenceInput(input *core.Input) any {
	if input == nil {
		return nil
	}
	return *input
}

// allowedMCPIDs returns the lowercased, deduplicated list of MCP IDs declared on the
// agent and workflow config for this execution context.
func (uc *ExecuteTask) allowedMCPIDs(agentConfig *agent.Config, input *ExecuteTaskInput) []string {
	allowed := make(map[string]struct{})
	// Agent-level MCPs
	if agentConfig != nil {
		for i := range agentConfig.MCPs {
			id := strings.ToLower(strings.TrimSpace(agentConfig.MCPs[i].ID))
			if id != "" {
				allowed[id] = struct{}{}
			}
		}
	}
	// Workflow-level MCPs
	if input != nil && input.WorkflowConfig != nil {
		mcps := input.WorkflowConfig.GetMCPs()
		for i := range mcps {
			id := strings.ToLower(strings.TrimSpace(mcps[i].ID))
			if id != "" {
				allowed[id] = struct{}{}
			}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	out := make([]string, 0, len(allowed))
	for id := range allowed {
		out = append(out, id)
	}
	return out
}
