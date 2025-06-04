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

	// Add parent context
	if parent := n.buildParentContext(ctx); parent != nil {
		context["parent"] = parent
	}

	// Add current input context
	if ctx.CurrentInput != nil {
		context["input"] = ctx.CurrentInput
	}

	// Add merged environment context
	if ctx.MergedEnv != nil {
		context["env"] = ctx.MergedEnv
	}

	return context
}

func (n *Normalizer) buildWorkflowContext(ctx *NormalizationContext) map[string]any {
	workflowContext := map[string]any{
		"id":     ctx.WorkflowState.WorkflowID,
		"input":  ctx.WorkflowState.Input,
		"output": ctx.WorkflowState.Output,
	}

	// Add workflow config properties if available
	if ctx.WorkflowConfig != nil {
		wfMap, err := core.ConfigAsMap(ctx.WorkflowConfig)
		if err == nil {
			// Merge config properties with runtime state
			for k, v := range wfMap {
				if k != "input" && k != "output" { // Don't override runtime state
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
			"id":     taskID,
			"input":  taskState.Input,
			"output": taskState.Output,
		}

		// Add task config properties if available
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
	taskConfigMap, err := core.ConfigAsMap(taskConfig)
	if err != nil {
		return
	}
	// Merge config properties with runtime state
	for k, v := range taskConfigMap {
		if k != "input" && k != "output" { // Don't override runtime state
			taskContext[k] = v
		}
	}
}

func (n *Normalizer) buildParentContext(ctx *NormalizationContext) map[string]any {
	if ctx.ParentConfig != nil {
		return ctx.ParentConfig
	}

	if ctx.ParentTaskConfig != nil {
		// If parent is a task, build parent context from task config
		parentMap, err := core.ConfigAsMap(ctx.ParentTaskConfig)
		if err != nil {
			return nil
		}

		// Also add runtime state if available
		if ctx.WorkflowState.Tasks != nil {
			if parentTaskState, exists := ctx.WorkflowState.Tasks[ctx.ParentTaskConfig.ID]; exists {
				parentMap["input"] = parentTaskState.Input
				parentMap["output"] = parentTaskState.Output
			}
		}

		return parentMap
	}

	return nil
}

// -----
// Configuration Normalization
// -----

func (n *Normalizer) NormalizeTaskConfig(config *task.Config, ctx *NormalizationContext) error {
	if config == nil {
		return nil
	}

	// Update current input in context
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}

	// Note: When a task is called by another task, set ParentTaskConfig instead of ParentConfig
	// The buildContext method will handle extracting the parent task's configuration and runtime state
	context := n.buildContext(ctx)

	// Normalize input (With field)
	if config.With != nil {
		if err := n.normalizeInput(config.With, context); err != nil {
			return fmt.Errorf("failed to normalize task config input: %w", err)
		}
	}

	// Normalize environment variables
	if err := n.normalizeEnvMap(&config.Env, context); err != nil {
		return fmt.Errorf("failed to normalize task config env: %w", err)
	}

	// Normalize string fields
	if err := n.normalizeStringField(&config.Action, context); err != nil {
		return fmt.Errorf("failed to normalize task config action: %w", err)
	}

	if err := n.normalizeStringField(&config.Condition, context); err != nil {
		return fmt.Errorf("failed to normalize task config condition: %w", err)
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

	// Update current input in context
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}

	context := n.buildContext(ctx)

	// Normalize input (With field)
	if config.With != nil {
		if err := n.normalizeInput(config.With, context); err != nil {
			return fmt.Errorf("failed to normalize agent config input: %w", err)
		}
	}

	// Normalize environment variables
	if err := n.normalizeEnvMap(&config.Env, context); err != nil {
		return fmt.Errorf("failed to normalize agent config env: %w", err)
	}

	// Normalize instructions (most important for agents)
	if err := n.normalizeStringField(&config.Instructions, context); err != nil {
		return fmt.Errorf("failed to normalize agent config instructions: %w", err)
	}

	// Normalize agent actions (only if actionID is provided and actions exist)
	if actionID != "" && len(config.Actions) > 0 {
		var aConfig *agent.ActionConfig
		for _, action := range config.Actions {
			if action.ID == actionID {
				aConfig = action
				break
			}
		}
		if aConfig == nil {
			return fmt.Errorf("agent action %s not found in agent config %s", actionID, config.ID)
		}
		mergedInput, err := ctx.CurrentInput.Merge(aConfig.GetInput())
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
				"input":        config.With,
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

func (n *Normalizer) NormalizeToolConfig(config *tool.Config, ctx *NormalizationContext) error {
	if config == nil {
		return nil
	}

	// Update current input in context
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}

	context := n.buildContext(ctx)

	// Normalize input (With field)
	if config.With != nil {
		if err := n.normalizeInput(config.With, context); err != nil {
			return fmt.Errorf("failed to normalize tool config input: %w", err)
		}
	}

	// Normalize environment variables
	if err := n.normalizeEnvMap(&config.Env, context); err != nil {
		return fmt.Errorf("failed to normalize tool config env: %w", err)
	}

	// Normalize string fields
	if err := n.normalizeStringField(&config.Execute, context); err != nil {
		return fmt.Errorf("failed to normalize tool config execute: %w", err)
	}

	if err := n.normalizeStringField(&config.Description, context); err != nil {
		return fmt.Errorf("failed to normalize tool config description: %w", err)
	}

	return nil
}

func (n *Normalizer) NormalizeAgentActionConfig(config *agent.ActionConfig, ctx *NormalizationContext) error {
	if config == nil {
		return nil
	}

	// Update current input in context
	if ctx.CurrentInput == nil && config.With != nil {
		ctx.CurrentInput = config.With
	}

	context := n.buildContext(ctx)

	// Normalize input (With field)
	if config.With != nil {
		if err := n.normalizeInput(config.With, context); err != nil {
			return fmt.Errorf("failed to normalize agent action config input: %w", err)
		}
	}

	// Normalize prompt (most important for actions)
	if err := n.normalizeStringField(&config.Prompt, context); err != nil {
		return fmt.Errorf("failed to normalize agent action config prompt: %w", err)
	}

	return nil
}

// -----
// Helper Methods
// -----

func (n *Normalizer) normalizeInput(input *core.Input, context map[string]any) error {
	if input == nil {
		return nil
	}

	for k, v := range *input {
		parsedValue, err := n.engine.ParseMap(v, context)
		if err != nil {
			return fmt.Errorf("failed to parse template in input[%s]: %w", k, err)
		}
		(*input)[k] = parsedValue
	}
	return nil
}

func (n *Normalizer) normalizeEnvMap(envMap *core.EnvMap, context map[string]any) error {
	if envMap == nil {
		return nil
	}

	for k, v := range *envMap {
		if tplengine.HasTemplate(v) {
			parsed, err := n.engine.RenderString(v, context)
			if err != nil {
				return fmt.Errorf("failed to parse template in env[%s]: %w", k, err)
			}
			(*envMap)[k] = parsed
		}
	}
	return nil
}

func (n *Normalizer) normalizeStringField(field *string, context map[string]any) error {
	if field == nil || *field == "" {
		return nil
	}

	if tplengine.HasTemplate(*field) {
		parsed, err := n.engine.RenderString(*field, context)
		if err != nil {
			return fmt.Errorf("failed to parse template in string field: %w", err)
		}
		*field = parsed
	}
	return nil
}
