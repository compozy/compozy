package normalizer

import (
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/tplengine"
)

type Normalizer struct {
	engine *tplengine.TemplateEngine
	*ContextBuilder
}

func New() *Normalizer {
	return &Normalizer{
		engine:         tplengine.NewEngine(tplengine.FormatJSON),
		ContextBuilder: NewContextBuilder(),
	}
}

// -----------------------------------------------------------------------------
// Configuration Normalization
// -----------------------------------------------------------------------------

func (n *Normalizer) NormalizeTaskConfig(config *task.Config, ctx *NormalizationContext) error {
	if config == nil {
		return nil
	}
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	if config.Type == task.TaskTypeParallel {
		return n.normalizeParallelTaskConfig(config, ctx)
	}
	return n.normalizeRegularTaskConfig(config, ctx)
}

func (n *Normalizer) normalizeParallelTaskConfig(config *task.Config, ctx *NormalizationContext) error {
	context := n.BuildContext(ctx)
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	filterFunc := n.createParallelTaskFilterFunc(config)
	if err := n.processTaskWithFilter(config, configMap, context, filterFunc); err != nil {
		return err
	}
	return n.NormalizeParallelSubTasks(config, ctx)
}

func (n *Normalizer) normalizeRegularTaskConfig(config *task.Config, ctx *NormalizationContext) error {
	context := n.BuildContext(ctx)
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	filterFunc := n.createRegularTaskFilterFunc(config)
	return n.processTaskWithFilter(config, configMap, context, filterFunc)
}

// createParallelTaskFilterFunc creates a filter function for parallel tasks
func (n *Normalizer) createParallelTaskFilterFunc(config *task.Config) func(string) bool {
	return func(k string) bool {
		if n.isCommonExcludedField(k) {
			return true
		}
		return n.isCollectionSpecificField(config, k)
	}
}

// createRegularTaskFilterFunc creates a filter function for regular tasks
func (n *Normalizer) createRegularTaskFilterFunc(config *task.Config) func(string) bool {
	return func(k string) bool {
		if n.isBasicExcludedField(k) {
			return true
		}
		return n.isCollectionSpecificField(config, k)
	}
}

// isCommonExcludedField checks if a field should be excluded for all task types
func (n *Normalizer) isCommonExcludedField(k string) bool {
	return k == "agent" || k == "tool" || k == "tasks" || k == "outputs" || k == inputKey || k == outputKey
}

// isBasicExcludedField checks if a field should be excluded for basic tasks
func (n *Normalizer) isBasicExcludedField(k string) bool {
	return k == "agent" || k == "tool" || k == "outputs" || k == inputKey || k == outputKey
}

// isCollectionSpecificField checks if a field is collection-specific and should be excluded
func (n *Normalizer) isCollectionSpecificField(config *task.Config, k string) bool {
	if config.Type != task.TaskTypeCollection {
		return false
	}
	return k == "items" || k == "filter" || k == "template" || k == "mode" ||
		k == "batch" || k == "continue_on_error" || k == "item_var" ||
		k == "index_var" || k == "stop_condition"
}

// processTaskWithFilter processes a task configuration with the given filter
func (n *Normalizer) processTaskWithFilter(
	config *task.Config,
	configMap map[string]any,
	context map[string]any,
	filterFunc func(string) bool,
) error {
	parsed, err := n.engine.ParseMapWithFilter(configMap, context, filterFunc)
	if err != nil {
		return fmt.Errorf("failed to normalize task config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	return nil
}

// NormalizeParallelSubTasks normalizes sub-tasks within a parallel task with proper parent context
func (n *Normalizer) NormalizeParallelSubTasks(parallelConfig *task.Config, ctx *NormalizationContext) error {
	if parallelConfig.Type != task.TaskTypeParallel {
		return nil
	}

	parentConfigMap, err := n.buildParentConfigMap(parallelConfig, ctx)
	if err != nil {
		return fmt.Errorf("failed to build parent config map: %w", err)
	}

	if err := n.normalizeSubTasksList(parallelConfig, parentConfigMap, ctx); err != nil {
		return err
	}

	return n.normalizeTaskReference(parallelConfig, parentConfigMap, ctx)
}

// buildParentConfigMap builds the parent configuration map for sub-tasks
func (n *Normalizer) buildParentConfigMap(
	parallelConfig *task.Config,
	ctx *NormalizationContext,
) (map[string]any, error) {
	parentConfigMap, err := parallelConfig.AsMap()
	if err != nil {
		return nil, err
	}

	n.addRuntimeStateToParentConfig(parentConfigMap, parallelConfig, ctx)
	n.addFallbackInputToParentConfig(parentConfigMap, parallelConfig)

	return parentConfigMap, nil
}

// addRuntimeStateToParentConfig adds runtime state to parent config if available
func (n *Normalizer) addRuntimeStateToParentConfig(
	parentConfigMap map[string]any,
	parallelConfig *task.Config,
	ctx *NormalizationContext,
) {
	if ctx.WorkflowState.Tasks == nil {
		return
	}

	taskState, exists := ctx.WorkflowState.Tasks[parallelConfig.ID]
	if !exists {
		return
	}

	parentConfigMap[inputKey] = taskState.Input
	parentConfigMap[outputKey] = taskState.Output
}

// addFallbackInputToParentConfig adds fallback input if no runtime state exists
func (n *Normalizer) addFallbackInputToParentConfig(
	parentConfigMap map[string]any,
	parallelConfig *task.Config,
) {
	if parentConfigMap[inputKey] == nil && parallelConfig.With != nil {
		parentConfigMap[inputKey] = parallelConfig.With
	}
}

// normalizeSubTasksList normalizes all sub-tasks in the parallel task
func (n *Normalizer) normalizeSubTasksList(
	parallelConfig *task.Config,
	parentConfigMap map[string]any,
	ctx *NormalizationContext,
) error {
	for i := range parallelConfig.Tasks {
		subTask := &parallelConfig.Tasks[i]
		subTaskCtx := n.createSubTaskContext(subTask, parentConfigMap, ctx)

		if err := n.NormalizeTaskConfig(subTask, subTaskCtx); err != nil {
			return fmt.Errorf("failed to normalize sub-task %s: %w", subTask.ID, err)
		}
	}
	return nil
}

// normalizeTaskReference normalizes the task reference if present
func (n *Normalizer) normalizeTaskReference(
	parallelConfig *task.Config,
	parentConfigMap map[string]any,
	ctx *NormalizationContext,
) error {
	if parallelConfig.Task == nil {
		return nil
	}

	subTaskCtx := n.createSubTaskContext(parallelConfig.Task, parentConfigMap, ctx)
	if err := n.NormalizeTaskConfig(parallelConfig.Task, subTaskCtx); err != nil {
		return fmt.Errorf("failed to normalize task reference: %w", err)
	}
	return nil
}

// createSubTaskContext creates a normalization context for a sub-task
func (n *Normalizer) createSubTaskContext(
	subTask *task.Config,
	parentConfigMap map[string]any,
	ctx *NormalizationContext,
) *NormalizationContext {
	return &NormalizationContext{
		WorkflowState:  ctx.WorkflowState,
		WorkflowConfig: ctx.WorkflowConfig,
		TaskConfigs:    ctx.TaskConfigs,
		ParentConfig:   parentConfigMap,
		CurrentInput:   subTask.With,
		MergedEnv:      ctx.MergedEnv,
	}
}

func (n *Normalizer) NormalizeAgentConfig(
	config *agent.Config,
	ctx *NormalizationContext,
	actionID string,
) error {
	if config == nil {
		return nil
	}
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	context := n.BuildContext(ctx)
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	parsed, err := n.engine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "actions" || k == "tools" || k == inputKey || k == outputKey
	})
	if err != nil {
		return fmt.Errorf("failed to normalize task config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	if err := n.NormalizeAgentActions(config, ctx, actionID); err != nil {
		return fmt.Errorf("failed to normalize agent actions: %w", err)
	}
	for _, toolConfig := range config.Tools {
		if err := n.NormalizeToolConfig(&toolConfig, ctx); err != nil {
			return fmt.Errorf("failed to normalize tool config: %w", err)
		}
	}
	return nil
}

func (n *Normalizer) NormalizeAgentActions(config *agent.Config, ctx *NormalizationContext, actionID string) error {
	// Normalize agent actions (only if actionID is provided and actions exist)
	if actionID != "" && len(config.Actions) > 0 {
		aConfig, err := agent.FindActionConfig(config.Actions, actionID)
		if err != nil {
			return fmt.Errorf("failed to find action config: %w", err)
		}
		if aConfig == nil {
			return fmt.Errorf("agent action %s not found in agent config %s", actionID, config.ID)
		}
		mergedInput, err := config.GetInput().Merge(aConfig.GetInput())
		if err != nil {
			return fmt.Errorf("failed to merge input for agent action %s: %w", aConfig.ID, err)
		}
		aConfig.With = mergedInput
		actionCtx := &NormalizationContext{
			WorkflowState:  ctx.WorkflowState,
			WorkflowConfig: ctx.WorkflowConfig,
			TaskConfigs:    ctx.TaskConfigs,
			ParentConfig: map[string]any{
				"id":           config.ID,
				inputKey:       config.With,
				"instructions": config.Instructions,
				"config":       config.Config,
			},
			CurrentInput: aConfig.With,
			MergedEnv:    ctx.MergedEnv,
		}
		if err := n.NormalizeAgentActionConfig(aConfig, actionCtx); err != nil {
			return fmt.Errorf("failed to normalize agent action config: %w", err)
		}
	}
	return nil
}

func (n *Normalizer) NormalizeAgentActionConfig(config *agent.ActionConfig, ctx *NormalizationContext) error {
	if config == nil {
		return nil
	}
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	context := n.BuildContext(ctx)
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	parsed, err := n.engine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == inputKey || k == outputKey
	})
	if err != nil {
		return fmt.Errorf("failed to normalize task config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}
	return nil
}

func (n *Normalizer) NormalizeToolConfig(config *tool.Config, ctx *NormalizationContext) error {
	if config == nil {
		return nil
	}
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}
	context := n.BuildContext(ctx)
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	parsed, err := n.engine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == inputKey || k == outputKey
	})
	if err != nil {
		return fmt.Errorf("failed to normalize task config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}

	return nil
}

func (n *Normalizer) NormalizeTransition(transition *core.SuccessTransition, ctx *NormalizationContext) error {
	if transition == nil {
		return nil
	}
	if ctx.CurrentInput == nil && transition.With != nil {
		ctx.CurrentInput = transition.With
	}
	context := n.BuildContext(ctx)
	configMap, err := transition.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert transition to map: %w", err)
	}
	parsed, err := n.engine.ParseMap(configMap, context)
	if err != nil {
		return fmt.Errorf("failed to normalize transition: %w", err)
	}
	if err := transition.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update transition from normalized map: %w", err)
	}
	return nil
}

func (n *Normalizer) NormalizeErrorTransition(transition *core.ErrorTransition, ctx *NormalizationContext) error {
	if transition == nil {
		return nil
	}
	if ctx.CurrentInput == nil && transition.With != nil {
		ctx.CurrentInput = transition.With
	}
	context := n.BuildContext(ctx)
	configMap, err := transition.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert transition to map: %w", err)
	}
	parsed, err := n.engine.ParseMap(configMap, context)
	if err != nil {
		return fmt.Errorf("failed to normalize transition: %w", err)
	}
	if err := transition.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update transition from normalized map: %w", err)
	}
	return nil
}
