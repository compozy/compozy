package normalizer

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

const (
	inputKey  = "input"
	outputKey = "output"
)

type ContextBuilder struct{}

func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{}
}

type NormalizationContext struct {
	WorkflowState    *workflow.State
	WorkflowConfig   *workflow.Config
	ParentConfig     map[string]any          // Parent configuration properties
	ParentTaskConfig *task.Config            // Parent task config when a task calls another task
	TaskConfigs      map[string]*task.Config // Task configurations by ID
	CurrentInput     *core.Input
	MergedEnv        *core.EnvMap
}

func (cb *ContextBuilder) BuildContext(ctx *NormalizationContext) map[string]any {
	context := map[string]any{
		"workflow": cb.buildWorkflowContext(ctx),
		"tasks":    cb.buildTasksContext(ctx),
	}
	if parent := cb.buildParentContext(ctx); parent != nil {
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

func (cb *ContextBuilder) buildWorkflowContext(ctx *NormalizationContext) map[string]any {
	workflowContext := map[string]any{}

	// Handle case where WorkflowState might be nil (e.g., during agent action normalization)
	if ctx.WorkflowState != nil {
		workflowContext["id"] = ctx.WorkflowState.WorkflowID
		workflowContext[inputKey] = ctx.WorkflowState.Input
		workflowContext[outputKey] = ctx.WorkflowState.Output
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

func (cb *ContextBuilder) buildTasksContext(ctx *NormalizationContext) map[string]any {
	tasksMap := make(map[string]any)

	// Handle case where WorkflowState might be nil or Tasks might be nil
	if ctx.WorkflowState == nil || ctx.WorkflowState.Tasks == nil {
		return tasksMap
	}

	for taskID, taskState := range ctx.WorkflowState.Tasks {
		taskContext := cb.buildSingleTaskContext(taskID, taskState, ctx)
		tasksMap[taskID] = taskContext
	}
	return tasksMap
}

func (cb *ContextBuilder) buildSingleTaskContext(
	taskID string,
	taskState *task.State,
	ctx *NormalizationContext,
) map[string]any {
	taskContext := map[string]any{
		"id":     taskID,
		inputKey: taskState.Input,
	}
	taskContext[outputKey] = cb.buildTaskOutput(taskState)
	cb.mergeTaskConfigIfExists(taskContext, taskID, ctx)
	return taskContext
}

func (cb *ContextBuilder) buildTaskOutput(taskState *task.State) any {
	if taskState.IsParallel() && taskState.ParallelState != nil {
		nestedOutput := make(map[string]any)
		subtasks := taskState.SubTasks
		for subTaskID, subTaskState := range subtasks {
			subTaskOutput := cb.buildTaskOutput(subTaskState)
			if subTaskOutput != nil {
				nestedOutput[subTaskID] = map[string]any{
					"output": subTaskOutput,
				}
			}
		}
		return nestedOutput
	}
	if taskState.Output != nil {
		return *taskState.Output
	}
	return nil
}

func (cb *ContextBuilder) mergeTaskConfigIfExists(
	taskContext map[string]any,
	taskID string,
	ctx *NormalizationContext,
) {
	if ctx.TaskConfigs != nil {
		if taskConfig, exists := ctx.TaskConfigs[taskID]; exists {
			cb.mergeTaskConfig(taskContext, taskConfig)
		}
	}
}

func (cb *ContextBuilder) mergeTaskConfig(taskContext map[string]any, taskConfig *task.Config) {
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

func (cb *ContextBuilder) buildParentContext(ctx *NormalizationContext) map[string]any {
	if ctx.ParentConfig != nil {
		return ctx.ParentConfig
	}
	if ctx.ParentTaskConfig != nil {
		parentMap, err := ctx.ParentTaskConfig.AsMap()
		if err != nil {
			return nil
		}
		// Handle case where WorkflowState might be nil
		if ctx.WorkflowState != nil && ctx.WorkflowState.Tasks != nil {
			if parentTaskState, exists := ctx.WorkflowState.Tasks[ctx.ParentTaskConfig.ID]; exists {
				parentMap[inputKey] = parentTaskState.Input
				parentMap[outputKey] = parentTaskState.Output
			}
		}
		return parentMap
	}
	return nil
}
