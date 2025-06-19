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
	// First normalize the parallel task itself (excluding the tasks field)
	parsed, err := n.engine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "agent" || k == "tool" || k == "tasks" || k == "outputs" || k == inputKey || k == outputKey
	})
	if err != nil {
		return fmt.Errorf("failed to normalize parallel task config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update parallel task config from normalized map: %w", err)
	}
	// Now normalize each sub-task with the parallel task as parent
	if err := n.NormalizeParallelSubTasks(config, ctx); err != nil {
		return fmt.Errorf("failed to normalize parallel sub-tasks: %w", err)
	}
	return nil
}

func (n *Normalizer) normalizeRegularTaskConfig(config *task.Config, ctx *NormalizationContext) error {
	context := n.BuildContext(ctx)
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}

	// Preserve existing With values before normalization
	existingWith := config.With

	parsed, err := n.engine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "agent" || k == "tool" || k == "outputs" || k == outputKey
	})
	if err != nil {
		return fmt.Errorf("failed to normalize task config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
	}

	// Merge existing With values back into the normalized config
	if existingWith != nil && config.With != nil {
		// Check for aliasing to prevent concurrent map iteration and write panic
		if existingWith != config.With {
			// Merge existing values into normalized values (existing takes precedence)
			for key, value := range *existingWith {
				(*config.With)[key] = value
			}
		}
	} else if existingWith != nil {
		// If normalization cleared With but we had existing values, restore them
		config.With = existingWith
	}

	return nil
}

// NormalizeParallelSubTasks normalizes sub-tasks within a parallel task with proper parent context
func (n *Normalizer) NormalizeParallelSubTasks(parallelConfig *task.Config, ctx *NormalizationContext) error {
	if parallelConfig.Type != task.TaskTypeParallel {
		return nil
	}
	// Build parent context from the parallel task configuration
	parentConfigMap, err := parallelConfig.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert parallel task config to map: %w", err)
	}
	// Add runtime state for the parallel task if available
	if ctx.WorkflowState.Tasks != nil {
		if taskState, exists := ctx.WorkflowState.Tasks[parallelConfig.ID]; exists {
			parentConfigMap[inputKey] = taskState.Input
			parentConfigMap[outputKey] = taskState.Output
		}
	}
	// If no runtime state exists, use the parallel task's config input as fallback
	if parentConfigMap[inputKey] == nil && parallelConfig.With != nil {
		parentConfigMap[inputKey] = parallelConfig.With
	}
	// Normalize each sub-task with the parallel task as parent
	for i := range parallelConfig.Tasks {
		subTask := &parallelConfig.Tasks[i]
		// Create context for sub-task with parallel task as parent
		subTaskCtx := &NormalizationContext{
			WorkflowState:  ctx.WorkflowState,
			WorkflowConfig: ctx.WorkflowConfig,
			TaskConfigs:    ctx.TaskConfigs,
			ParentConfig:   parentConfigMap,
			CurrentInput:   subTask.With,
			MergedEnv:      ctx.MergedEnv,
		}
		// Recursively normalize the sub-task (this handles nested parallel tasks too)
		if err := n.NormalizeTaskConfig(subTask, subTaskCtx); err != nil {
			return fmt.Errorf("failed to normalize sub-task %s: %w", subTask.ID, err)
		}
	}
	// Also normalize the task reference if present
	if parallelConfig.Task != nil {
		subTaskCtx := &NormalizationContext{
			WorkflowState:  ctx.WorkflowState,
			WorkflowConfig: ctx.WorkflowConfig,
			TaskConfigs:    ctx.TaskConfigs,
			ParentConfig:   parentConfigMap,
			CurrentInput:   parallelConfig.Task.With,
			MergedEnv:      ctx.MergedEnv,
		}
		if err := n.NormalizeTaskConfig(parallelConfig.Task, subTaskCtx); err != nil {
			return fmt.Errorf("failed to normalize task reference: %w", err)
		}
	}
	return nil
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
	for i := range config.Tools {
		if err := n.NormalizeToolConfig(&config.Tools[i], ctx); err != nil {
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
