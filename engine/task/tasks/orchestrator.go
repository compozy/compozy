package tasks

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/tasks/wait"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

// ConfigOrchestrator handles the orchestration of normalization processes
type ConfigOrchestrator struct {
	factory        Factory
	contextBuilder *shared.ContextBuilder
}

// NewConfigOrchestrator creates a new configuration orchestrator
func NewConfigOrchestrator(ctx context.Context, factory Factory) (*ConfigOrchestrator, error) {
	builder, err := shared.NewContextBuilder(ctx)
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
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) error {
	allTaskConfigsMap := BuildTaskConfigsMap(workflowConfig.Tasks)
	normCtx := o.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	normCtx.TaskConfigs = allTaskConfigsMap
	normCtx.ParentConfig = map[string]any{
		"id":     workflowState.WorkflowID,
		"input":  workflowState.Input,
		"output": workflowState.Output,
	}
	normCtx.CurrentInput = taskConfig.With // Set the task's With field as current input
	normCtx.MergedEnv = taskConfig.Env     // Assume env is already merged at this point
	if taskConfig.With != nil {
		if normCtx.Variables == nil {
			normCtx.Variables = make(map[string]any)
		}
		o.contextBuilder.VariableBuilder.AddCurrentInputToVariables(normCtx.Variables, taskConfig.With)
	}
	normalizer, err := o.factory.CreateNormalizer(ctx, taskConfig.Type)
	if err != nil {
		return fmt.Errorf("failed to create normalizer for task %s: %w", taskConfig.ID, err)
	}
	if err := normalizer.Normalize(ctx, taskConfig, normCtx); err != nil {
		return fmt.Errorf("failed to normalize task config for %s: %w", taskConfig.ID, err)
	}
	return nil
}

// NormalizeAgentComponent normalizes an agent component configuration
func (o *ConfigOrchestrator) NormalizeAgentComponent(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	agentConfig *agent.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	parentConfig, err := o.buildParentComponentConfig(workflowState, taskConfig)
	if err != nil {
		return err
	}
	if err := o.mergeAgentInput(taskConfig, agentConfig); err != nil {
		return err
	}
	normCtx := o.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	o.prepareComponentContext(normCtx, allTaskConfigs, parentConfig, agentConfig.With, agentConfig.Env)
	agentNormalizer := o.factory.CreateAgentNormalizer()
	if err := agentNormalizer.NormalizeAgent(agentConfig, normCtx, taskConfig.Action); err != nil {
		return fmt.Errorf("failed to normalize agent config for %s: %w", agentConfig.ID, err)
	}
	if err := agentConfig.NormalizeAndValidateMemoryConfig(); err != nil {
		return fmt.Errorf("failed to process memory config for agent %s: %w", agentConfig.ID, err)
	}
	return nil
}

// NormalizeToolComponent normalizes a tool component configuration
func (o *ConfigOrchestrator) NormalizeToolComponent(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
	toolConfig *tool.Config,
	allTaskConfigs map[string]*task.Config,
) error {
	parentConfig, err := o.buildParentComponentConfig(workflowState, taskConfig)
	if err != nil {
		return err
	}
	if err := o.mergeToolInput(taskConfig, toolConfig); err != nil {
		return err
	}
	normCtx := o.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	o.prepareComponentContext(normCtx, allTaskConfigs, parentConfig, toolConfig.With, toolConfig.Env)
	toolNormalizer := o.factory.CreateToolNormalizer()
	if err := toolNormalizer.NormalizeTool(toolConfig, normCtx); err != nil {
		return fmt.Errorf("failed to normalize tool config for %s: %w", toolConfig.ID, err)
	}
	return nil
}

// buildParentComponentConfig assembles parent configuration data with runtime state.
func (o *ConfigOrchestrator) buildParentComponentConfig(
	workflowState *workflow.State,
	taskConfig *task.Config,
) (map[string]any, error) {
	parentConfig, err := core.AsMapDefault(taskConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to convert task config to map: %w", err)
	}
	if workflowState.Tasks != nil {
		if taskState, exists := workflowState.Tasks[taskConfig.ID]; exists {
			parentConfig["input"] = taskState.Input
			parentConfig["output"] = taskState.Output
		}
	}
	return parentConfig, nil
}

// mergeAgentInput merges task and agent inputs before normalization.
func (o *ConfigOrchestrator) mergeAgentInput(taskConfig *task.Config, agentConfig *agent.Config) error {
	mergedInput, err := taskConfig.GetInput().Merge(agentConfig.GetInput())
	if err != nil {
		return fmt.Errorf("failed to merge input for agent %s in task %s: %w", agentConfig.ID, taskConfig.ID, err)
	}
	agentConfig.With = mergedInput
	return nil
}

// mergeToolInput merges task and tool inputs before normalization.
func (o *ConfigOrchestrator) mergeToolInput(taskConfig *task.Config, toolConfig *tool.Config) error {
	mergedInput, err := taskConfig.GetInput().Merge(toolConfig.GetInput())
	if err != nil {
		return fmt.Errorf("failed to merge input for tool %s in task %s: %w", toolConfig.ID, taskConfig.ID, err)
	}
	toolConfig.With = mergedInput
	return nil
}

// prepareComponentContext configures normalization context with shared component data.
func (o *ConfigOrchestrator) prepareComponentContext(
	normCtx *shared.NormalizationContext,
	allTaskConfigs map[string]*task.Config,
	parentConfig map[string]any,
	componentInput *core.Input,
	componentEnv *core.EnvMap,
) {
	normCtx.TaskConfigs = allTaskConfigs
	normCtx.ParentConfig = parentConfig
	normCtx.CurrentInput = componentInput
	normCtx.MergedEnv = componentEnv
	o.ensureContextVariables(normCtx)
	if componentInput != nil {
		o.contextBuilder.VariableBuilder.AddCurrentInputToVariables(normCtx.Variables, componentInput)
	}
	normCtx.Variables["parent"] = parentConfig
}

// ensureContextVariables initializes the variables map when absent.
func (o *ConfigOrchestrator) ensureContextVariables(normCtx *shared.NormalizationContext) {
	if normCtx.Variables == nil {
		normCtx.Variables = make(map[string]any)
	}
}

// NormalizeSuccessTransition normalizes a success transition configuration
func (o *ConfigOrchestrator) NormalizeSuccessTransition(
	ctx context.Context,
	transition *core.SuccessTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	mergedEnv *core.EnvMap,
) error {
	if transition == nil {
		return nil
	}
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return fmt.Errorf("failed to convert workflow config to map: %w", err)
	}
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output
	normCtx := o.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, nil)
	normCtx.TaskConfigs = allTaskConfigs
	normCtx.ParentConfig = parentConfig
	normCtx.CurrentInput = transition.With
	normCtx.MergedEnv = mergedEnv
	if transition.With != nil {
		if normCtx.Variables == nil {
			normCtx.Variables = make(map[string]any)
		}
		o.contextBuilder.VariableBuilder.AddCurrentInputToVariables(normCtx.Variables, transition.With)
	}
	transitionNormalizer := o.factory.CreateSuccessTransitionNormalizer()
	if err := transitionNormalizer.Normalize(transition, normCtx); err != nil {
		return fmt.Errorf("failed to normalize success transition: %w", err)
	}
	return nil
}

// NormalizeErrorTransition normalizes an error transition configuration
func (o *ConfigOrchestrator) NormalizeErrorTransition(
	ctx context.Context,
	transition *core.ErrorTransition,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	allTaskConfigs map[string]*task.Config,
	mergedEnv *core.EnvMap,
) error {
	if transition == nil {
		return nil
	}
	parentConfig, err := core.AsMapDefault(workflowConfig)
	if err != nil {
		return fmt.Errorf("failed to convert workflow config to map: %w", err)
	}
	parentConfig["input"] = workflowState.Input
	parentConfig["output"] = workflowState.Output
	normCtx := o.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, nil)
	normCtx.TaskConfigs = allTaskConfigs
	normCtx.ParentConfig = parentConfig
	normCtx.CurrentInput = transition.With
	normCtx.MergedEnv = mergedEnv
	if transition.With != nil {
		if normCtx.Variables == nil {
			normCtx.Variables = make(map[string]any)
		}
		o.contextBuilder.VariableBuilder.AddCurrentInputToVariables(normCtx.Variables, transition.With)
	}
	transitionNormalizer := o.factory.CreateErrorTransitionNormalizer()
	if err := transitionNormalizer.Normalize(transition, normCtx); err != nil {
		return fmt.Errorf("failed to normalize error transition: %w", err)
	}
	return nil
}

// NormalizeTaskOutput applies output transformation to task output
func (o *ConfigOrchestrator) NormalizeTaskOutput(
	ctx context.Context,
	taskOutput *core.Output,
	outputsConfig *core.Input,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) (*core.Output, error) {
	if outputsConfig == nil || taskOutput == nil {
		return taskOutput, nil
	}
	taskConfigs := BuildTaskConfigsMap(workflowConfig.Tasks)
	normCtx := o.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	normCtx.TaskConfigs = taskConfigs
	normCtx.CurrentInput = taskConfig.With
	normCtx.MergedEnv = taskConfig.Env
	if taskConfig.With != nil {
		if normCtx.Variables == nil {
			normCtx.Variables = make(map[string]any)
		}
		o.contextBuilder.VariableBuilder.AddCurrentInputToVariables(normCtx.Variables, taskConfig.With)
	}
	transformer := o.factory.CreateOutputTransformer()
	return transformer.TransformOutput(ctx, taskOutput, outputsConfig, normCtx, taskConfig)
}

// NormalizeTaskWithSignal normalizes a task config with signal context (for wait tasks)
func (o *ConfigOrchestrator) NormalizeTaskWithSignal(
	ctx context.Context,
	config *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	signal any,
) error {
	allTaskConfigsMap := BuildTaskConfigsMap(workflowConfig.Tasks)
	normCtx := o.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, config)
	normCtx.TaskConfigs = allTaskConfigsMap
	normCtx.ParentConfig = map[string]any{
		"id":     workflowState.WorkflowID,
		"input":  workflowState.Input,
		"output": workflowState.Output,
	}
	normCtx.MergedEnv = config.Env
	// Note: Wait task processors can be any task type (usually basic) but still need signal context
	normalizer, err := o.factory.CreateNormalizer(ctx, task.TaskTypeWait)
	if err != nil {
		return fmt.Errorf("failed to create wait normalizer: %w", err)
	}
	waitNormalizer, ok := normalizer.(*wait.Normalizer)
	if !ok {
		return fmt.Errorf("normalizer is not a wait normalizer")
	}
	return waitNormalizer.NormalizeWithSignal(ctx, config, normCtx, signal)
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
