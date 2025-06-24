package task2

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/task2/wait"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

// ConfigOrchestrator handles the orchestration of normalization processes
type ConfigOrchestrator struct {
	factory        NormalizerFactory
	contextBuilder *shared.ContextBuilder
}

// NewConfigOrchestrator creates a new configuration orchestrator
func NewConfigOrchestrator(factory NormalizerFactory) (*ConfigOrchestrator, error) {
	builder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	return &ConfigOrchestrator{
		factory:        factory,
		contextBuilder: builder,
	}, nil
}

// NormalizeTask normalizes a task configuration
func (o *ConfigOrchestrator) NormalizeTask(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) error {
	// Build task configs map
	allTaskConfigsMap := BuildTaskConfigsMap(workflowConfig.Tasks)
	// Build template variables and create normalization context
	normCtx := o.contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	// Set additional fields
	normCtx.TaskConfigs = allTaskConfigsMap
	normCtx.ParentConfig = map[string]any{
		"id":     workflowState.WorkflowID,
		"input":  workflowState.Input,
		"output": workflowState.Output,
	}
	normCtx.MergedEnv = taskConfig.Env // Assume env is already merged at this point
	// Get task normalizer
	normalizer, err := o.factory.CreateNormalizer(taskConfig.Type)
	if err != nil {
		return fmt.Errorf("failed to create normalizer for task %s: %w", taskConfig.ID, err)
	}
	// Normalize the task
	if err := normalizer.Normalize(taskConfig, normCtx); err != nil {
		return fmt.Errorf("failed to normalize task config for %s: %w", taskConfig.ID, err)
	}
	return nil
}

// NormalizeAgentComponent normalizes an agent component configuration
func (o *ConfigOrchestrator) NormalizeAgentComponent(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	agentConfig *agent.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	// Build complete parent context with all task config properties
	parentConfig, err := core.AsMapDefault(taskConfig)
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// Add runtime state if available
	if workflowState.Tasks != nil {
		if taskState, exists := workflowState.Tasks[taskConfig.ID]; exists {
			parentConfig["input"] = taskState.Input
			parentConfig["output"] = taskState.Output
		}
	}
	// Merge input from task and agent
	mergedInput, err := taskConfig.GetInput().Merge(agentConfig.GetInput())
	if err != nil {
		return fmt.Errorf("failed to merge input for agent %s in task %s: %w", agentConfig.ID, taskConfig.ID, err)
	}
	agentConfig.With = mergedInput
	// Build template variables and create normalization context
	normCtx := o.contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	// Set additional fields
	normCtx.TaskConfigs = allTaskConfigs
	normCtx.ParentConfig = parentConfig
	normCtx.CurrentInput = agentConfig.With
	normCtx.MergedEnv = agentConfig.Env // Assume env is already merged
	// Add parent context to variables for template processing
	if normCtx.Variables == nil {
		normCtx.Variables = make(map[string]any)
	}
	normCtx.Variables["parent"] = parentConfig
	// Get agent normalizer
	agentNormalizer := o.factory.CreateAgentNormalizer()
	// Normalize the agent
	if err := agentNormalizer.NormalizeAgent(agentConfig, normCtx, taskConfig.Action); err != nil {
		return fmt.Errorf("failed to normalize agent config for %s: %w", agentConfig.ID, err)
	}
	return nil
}

// NormalizeToolComponent normalizes a tool component configuration
func (o *ConfigOrchestrator) NormalizeToolComponent(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	toolConfig *tool.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	// Build parent context
	parentConfig, err := core.AsMapDefault(taskConfig)
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// Add runtime state if available
	if workflowState.Tasks != nil {
		if taskState, exists := workflowState.Tasks[taskConfig.ID]; exists {
			parentConfig["input"] = taskState.Input
			parentConfig["output"] = taskState.Output
		}
	}
	// Merge input from task and tool
	mergedInput, err := taskConfig.GetInput().Merge(toolConfig.GetInput())
	if err != nil {
		return fmt.Errorf("failed to merge input for tool %s in task %s: %w", toolConfig.ID, taskConfig.ID, err)
	}
	toolConfig.With = mergedInput
	// Build template variables and create normalization context
	normCtx := o.contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	// Set additional fields
	normCtx.TaskConfigs = allTaskConfigs
	normCtx.ParentConfig = parentConfig
	normCtx.CurrentInput = toolConfig.With
	normCtx.MergedEnv = toolConfig.Env // Assume env is already merged
	// Add parent context to variables for template processing
	if normCtx.Variables == nil {
		normCtx.Variables = make(map[string]any)
	}
	normCtx.Variables["parent"] = parentConfig
	// Get tool normalizer
	toolNormalizer := o.factory.CreateToolNormalizer()
	// Normalize the tool
	if err := toolNormalizer.NormalizeTool(toolConfig, normCtx); err != nil {
		return fmt.Errorf("failed to normalize tool config for %s: %w", toolConfig.ID, err)
	}
	return nil
}

// NormalizeSuccessTransition normalizes a success transition configuration
func (o *ConfigOrchestrator) NormalizeSuccessTransition(
	transition *core.SuccessTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	mergedEnv *core.EnvMap,
) error {
	if transition == nil {
		return nil
	}
	// Build complete parent context with all workflow config properties
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return fmt.Errorf("failed to convert workflow config to map: %w", err)
	}
	// Add workflow runtime state
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output
	// Build template variables and create normalization context
	normCtx := o.contextBuilder.BuildContext(workflowState, workflowConfig, nil)
	// Set additional fields
	normCtx.TaskConfigs = allTaskConfigs
	normCtx.ParentConfig = parentConfig
	normCtx.CurrentInput = transition.With
	normCtx.MergedEnv = mergedEnv
	// Get transition normalizer
	transitionNormalizer := o.factory.CreateSuccessTransitionNormalizer()
	// Normalize the transition
	if err := transitionNormalizer.Normalize(transition, normCtx); err != nil {
		return fmt.Errorf("failed to normalize success transition: %w", err)
	}
	return nil
}

// NormalizeErrorTransition normalizes an error transition configuration
func (o *ConfigOrchestrator) NormalizeErrorTransition(
	transition *core.ErrorTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	mergedEnv *core.EnvMap,
) error {
	if transition == nil {
		return nil
	}
	// Build complete parent context with all workflow config properties
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return fmt.Errorf("failed to convert workflow config to map: %w", err)
	}
	// Add workflow runtime state
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output
	// Build template variables and create normalization context
	normCtx := o.contextBuilder.BuildContext(workflowState, workflowConfig, nil)
	// Set additional fields
	normCtx.TaskConfigs = allTaskConfigs
	normCtx.ParentConfig = parentConfig
	normCtx.CurrentInput = transition.With
	normCtx.MergedEnv = mergedEnv
	// Get transition normalizer
	transitionNormalizer := o.factory.CreateErrorTransitionNormalizer()
	// Normalize the transition
	if err := transitionNormalizer.Normalize(transition, normCtx); err != nil {
		return fmt.Errorf("failed to normalize error transition: %w", err)
	}
	return nil
}

// NormalizeTaskOutput applies output transformation to task output
func (o *ConfigOrchestrator) NormalizeTaskOutput(
	taskOutput *core.Output,
	outputsConfig *core.Input,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*core.Output, error) {
	if outputsConfig == nil || taskOutput == nil {
		return taskOutput, nil
	}
	// Build task configs map
	taskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)
	// Build transformation context
	normCtx := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		TaskConfigs:    taskConfigs,
		CurrentInput:   taskConfig.With,
		MergedEnv:      taskConfig.Env,
	}
	// Build children index
	o.contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	// Get output transformer
	transformer := o.factory.CreateOutputTransformer()
	// Transform the output
	return transformer.TransformOutput(taskOutput, outputsConfig, normCtx, taskConfig)
}

// NormalizeWorkflowOutput transforms the workflow output using the outputs configuration
func (o *ConfigOrchestrator) NormalizeWorkflowOutput(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	outputsConfig *core.Output,
) (*core.Output, error) {
	if outputsConfig == nil {
		return nil, nil
	}
	log := logger.FromContext(ctx)
	// Build complete parent context with all workflow config properties
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to convert workflow config to map: %w", err)
	}
	// Add workflow runtime state
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output
	taskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)
	normCtx := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    taskConfigs,
		ParentConfig:   parentConfig,
		CurrentInput:   workflowState.Input,
		MergedEnv:      &[]core.EnvMap{workflowConfig.GetEnv()}[0],
	}
	// Log template processing start at debug level
	log.Debug("Starting workflow output template processing",
		"workflow_id", workflowState.WorkflowID,
		"workflow_exec_id", workflowState.WorkflowExecID,
		"task_count", len(workflowState.Tasks),
		"output_fields", len(*outputsConfig))
	// Get output transformer
	transformer := o.factory.CreateOutputTransformer()
	// Transform the output
	transformedOutput, err := transformer.TransformWorkflowOutput(workflowState, outputsConfig, normCtx)
	if err != nil {
		log.Error("Failed to transform workflow output",
			"workflow_id", workflowState.WorkflowID,
			"error", err)
		return nil, err
	}
	log.Debug("Successfully transformed workflow output",
		"workflow_id", workflowState.WorkflowID,
		"fields_count", len(*transformedOutput))
	return transformedOutput, nil
}

// NormalizeTaskWithSignal normalizes a task config with signal context (for wait tasks)
func (o *ConfigOrchestrator) NormalizeTaskWithSignal(
	config *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	signal any,
) error {
	// Build task configs map
	allTaskConfigsMap := BuildTaskConfigsMap(workflowConfig.Tasks)
	// Create normalization context
	normCtx := &shared.NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     config,
		TaskConfigs:    allTaskConfigsMap,
		ParentConfig: map[string]any{
			"id":     workflowState.WorkflowID,
			"input":  workflowState.Input,
			"output": workflowState.Output,
		},
		MergedEnv: config.Env,
	}
	// Build template variables
	o.contextBuilder.BuildContext(workflowState, workflowConfig, config)
	// Get wait task normalizer
	if config.Type != task.TaskTypeWait {
		return fmt.Errorf("signal normalization only supported for wait tasks, got: %s", config.Type)
	}
	normalizer, err := o.factory.CreateNormalizer(task.TaskTypeWait)
	if err != nil {
		return fmt.Errorf("failed to create wait normalizer: %w", err)
	}
	// Type assert to wait normalizer to access signal method
	waitNormalizer, ok := normalizer.(*wait.Normalizer)
	if !ok {
		return fmt.Errorf("normalizer is not a wait normalizer")
	}
	// Normalize with signal
	return waitNormalizer.NormalizeWithSignal(config, normCtx, signal)
}

// ClearCache clears the parent context cache - should be called at workflow start
func (o *ConfigOrchestrator) ClearCache() {
	o.contextBuilder.ClearCache()
}

// BuildTaskConfigsMap converts a slice of task.Config into a map keyed by task ID
func BuildTaskConfigsMap(taskConfigSlice []task.Config) map[string]*task.Config {
	configs := make(map[string]*task.Config)
	for i := range taskConfigSlice {
		tc := &taskConfigSlice[i]
		configs[tc.ID] = tc
	}
	return configs
}
