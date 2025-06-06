package normalizer

import (
	"fmt"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

const (
	inputKey  = "input"
	outputKey = "output"
)

type Normalizer struct {
	engine *tplengine.TemplateEngine
}

func New() *Normalizer {
	return &Normalizer{
		engine: tplengine.NewEngine(tplengine.FormatJSON),
	}
}

type NormalizationContext struct {
	WorkflowState    *workflow.State
	WorkflowConfig   *workflow.Config
	ParentConfig     map[string]any          // Parent configuration properties
	ParentTaskConfig *task.Config            // Parent task config when a task calls another task
	TaskConfigs      map[string]*task.Config // Task configurations by ID
	CurrentInput     *core.Input
	MergedEnv        core.EnvMap
}

func (n *Normalizer) buildContext(ctx *NormalizationContext) map[string]any {
	context := map[string]any{
		"workflow": n.buildWorkflowContext(ctx),
		"tasks":    n.buildTasksContext(ctx),
	}
	if parent := n.buildParentContext(ctx); parent != nil {
		context["parent"] = parent
	}
	if ctx.CurrentInput != nil {
		context[inputKey] = ctx.CurrentInput
	}
	if ctx.MergedEnv != nil {
		context["env"] = ctx.MergedEnv
	}
	return context
}

func (n *Normalizer) buildWorkflowContext(ctx *NormalizationContext) map[string]any {
	workflowContext := map[string]any{
		"id":      ctx.WorkflowState.WorkflowID,
		inputKey:  ctx.WorkflowState.Input,
		outputKey: ctx.WorkflowState.Output,
	}
	if ctx.WorkflowConfig != nil {
		wfMap, err := core.AsMapDefault(ctx.WorkflowConfig)
		if err == nil {
			for k, v := range wfMap {
				if k != inputKey && k != outputKey { // Don't override runtime state
					workflowContext[k] = v
				}
			}
		}
	}
	return workflowContext
}

func (n *Normalizer) buildTasksContext(ctx *NormalizationContext) map[string]any {
	tasksMap := make(map[string]any)
	if ctx.WorkflowState.Tasks == nil {
		return tasksMap
	}
	for taskID, taskState := range ctx.WorkflowState.Tasks {
		taskContext := map[string]any{
			"id":      taskID,
			inputKey:  taskState.Input,
			outputKey: taskState.Output,
		}
		if ctx.TaskConfigs != nil {
			if taskConfig, exists := ctx.TaskConfigs[taskID]; exists {
				n.mergeTaskConfig(taskContext, taskConfig)
			}
		}
		tasksMap[taskID] = taskContext
	}
	return tasksMap
}

func (n *Normalizer) mergeTaskConfig(taskContext map[string]any, taskConfig *task.Config) {
	taskConfigMap, err := taskConfig.AsMap()
	if err != nil {
		return
	}
	for k, v := range taskConfigMap {
		if k != inputKey && k != outputKey { // Don't override runtime state
			taskContext[k] = v
		}
	}
}

func (n *Normalizer) buildParentContext(ctx *NormalizationContext) map[string]any {
	if ctx.ParentConfig != nil {
		return ctx.ParentConfig
	}
	if ctx.ParentTaskConfig != nil {
		parentMap, err := ctx.ParentTaskConfig.AsMap()
		if err != nil {
			return nil
		}
		if ctx.WorkflowState.Tasks != nil {
			if parentTaskState, exists := ctx.WorkflowState.Tasks[ctx.ParentTaskConfig.ID]; exists {
				parentMap[inputKey] = parentTaskState.Input
				parentMap[outputKey] = parentTaskState.Output
			}
		}
		return parentMap
	}
	return nil
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
	context := n.buildContext(ctx)
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	parsed, err := n.engine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == "agent" || k == "tool" || k == inputKey || k == outputKey
	})
	if err != nil {
		return fmt.Errorf("failed to normalize task config: %w", err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update task config from normalized map: %w", err)
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
	context := n.buildContext(ctx)
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
		aConfig := agent.FindActionConfig(config.Actions, actionID)
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
	context := n.buildContext(ctx)
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
	context := n.buildContext(ctx)
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
	context := n.buildContext(ctx)
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
	context := n.buildContext(ctx)
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
