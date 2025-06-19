package normalizer

import (
	"maps"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

const (
	inputKey    = "input"
	outputKey   = "output"
	statusKey   = "status"
	errorKey    = "error"
	idKey       = "id"
	workflowKey = "workflow"
	tasksKey    = "tasks"
	parentKey   = "parent"
	itemKey     = "item"
	indexKey    = "index"
	envKey      = "env"
	childrenKey = "children"
)

const maxContextDepth = 10 // Prevent excessive recursion

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
	ChildrenIndex    map[string][]string // Maps parent task exec ID to child task IDs
}

func (cb *ContextBuilder) BuildContext(ctx *NormalizationContext) map[string]any {
	cb.buildChildrenIndex(ctx)
	context := map[string]any{
		workflowKey: cb.buildWorkflowContext(ctx),
		tasksKey:    cb.buildTasksContext(ctx),
	}
	if parent := cb.buildParentContext(ctx); parent != nil {
		context[parentKey] = parent
	}
	if ctx.CurrentInput != nil {
		context[inputKey] = ctx.CurrentInput
		// Also add item and index at top level for collection tasks
		if item, exists := (*ctx.CurrentInput)[itemKey]; exists {
			context[itemKey] = item
		}
		if index, exists := (*ctx.CurrentInput)[indexKey]; exists {
			context[indexKey] = index
		}
	}
	if ctx.MergedEnv != nil {
		context[envKey] = ctx.MergedEnv
	}
	return context
}

func (cb *ContextBuilder) buildWorkflowContext(ctx *NormalizationContext) map[string]any {
	var workflowContext map[string]any
	if ctx.WorkflowConfig != nil {
		// Start with config properties
		var err error
		workflowContext, err = core.AsMapDefault(ctx.WorkflowConfig)
		if err != nil {
			workflowContext = make(map[string]any)
		}
	}
	if workflowContext == nil {
		workflowContext = make(map[string]any)
	}
	// Overwrite/add runtime properties, which is safer and clearer
	workflowContext[idKey] = ctx.WorkflowState.WorkflowID
	workflowContext[inputKey] = ctx.WorkflowState.Input
	workflowContext[outputKey] = ctx.WorkflowState.Output
	return workflowContext
}

func (cb *ContextBuilder) buildTasksContext(ctx *NormalizationContext) map[string]any {
	tasksMap := make(map[string]any)
	if ctx.WorkflowState.Tasks == nil {
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
		idKey:     taskID,
		inputKey:  taskState.Input,
		statusKey: taskState.Status,
	}
	if taskState.Error != nil {
		taskContext[errorKey] = taskState.Error
	}
	taskContext[outputKey] = cb.buildTaskOutput(taskState, ctx, 0)
	if taskState.CanHaveChildren() && ctx.ChildrenIndex != nil {
		taskContext[childrenKey] = cb.BuildChildrenContext(taskState, ctx, 0)
	}
	cb.mergeTaskConfigIfExists(taskContext, taskID, ctx)
	return taskContext
}

func (cb *ContextBuilder) buildTaskOutput(taskState *task.State, ctx *NormalizationContext, depth int) any {
	// Prevent unbounded recursion
	if depth >= maxContextDepth {
		return nil
	}
	if taskState.CanHaveChildren() {
		// For parent tasks (parallel or collection), build nested output structure with child task outputs
		nestedOutput := make(map[string]any)
		// Include the parent's own output first (if any)
		if taskState.Output != nil {
			nestedOutput[outputKey] = *taskState.Output
		}
		// Use pre-built children index for O(1) lookup instead of O(n) scan
		if ctx != nil && ctx.ChildrenIndex != nil {
			parentTaskExecID := string(taskState.TaskExecID)
			if childTaskIDs, exists := ctx.ChildrenIndex[parentTaskExecID]; exists {
				for _, childTaskID := range childTaskIDs {
					if childTaskState, exists := ctx.WorkflowState.Tasks[childTaskID]; exists {
						// Add child task output to nested structure
						childOutput := make(map[string]any)
						childOutput[outputKey] = cb.buildTaskOutput(
							childTaskState,
							ctx,
							depth+1,
						) // Recursive call with depth
						childOutput[statusKey] = childTaskState.Status
						if childTaskState.Error != nil {
							childOutput[errorKey] = childTaskState.Error
						}
						nestedOutput[childTaskID] = childOutput
					}
				}
			}
		}
		return nestedOutput
	}
	if taskState.Output != nil {
		return *taskState.Output
	}
	return core.Output{}
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

func (cb *ContextBuilder) buildChildrenIndex(ctx *NormalizationContext) {
	if ctx.WorkflowState == nil || ctx.WorkflowState.Tasks == nil {
		ctx.ChildrenIndex = make(map[string][]string)
		return
	}
	ctx.ChildrenIndex = make(map[string][]string)
	for taskID, taskState := range ctx.WorkflowState.Tasks {
		if taskState.ParentStateID != nil {
			parentExecID := string(*taskState.ParentStateID)
			ctx.ChildrenIndex[parentExecID] = append(ctx.ChildrenIndex[parentExecID], taskID)
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

func (cb *ContextBuilder) BuildChildrenContext(
	parentState *task.State,
	ctx *NormalizationContext,
	depth int,
) map[string]any {
	if depth >= maxContextDepth {
		return make(map[string]any)
	}
	children := make(map[string]any)
	parentExecID := string(parentState.TaskExecID)
	if childTaskIDs, exists := ctx.ChildrenIndex[parentExecID]; exists {
		for _, childTaskID := range childTaskIDs {
			if childState, exists := ctx.WorkflowState.Tasks[childTaskID]; exists {
				children[childTaskID] = cb.buildChildContextWithoutParent(
					childTaskID,
					childState,
					ctx,
					depth+1,
				)
			}
		}
	}
	return children
}

func (cb *ContextBuilder) buildChildContextWithoutParent(
	taskID string,
	taskState *task.State,
	ctx *NormalizationContext,
	depth int,
) map[string]any {
	taskContext := map[string]any{
		idKey:     taskID,
		inputKey:  taskState.Input,
		statusKey: taskState.Status,
	}
	if taskState.Error != nil {
		taskContext[errorKey] = taskState.Error
	}
	// Ensure output key is always present for consistency
	if taskState.Output != nil {
		taskContext[outputKey] = *taskState.Output
	} else {
		taskContext[outputKey] = core.Output{}
	}
	if taskState.CanHaveChildren() && ctx.ChildrenIndex != nil {
		taskContext[childrenKey] = cb.BuildChildrenContext(taskState, ctx, depth)
	}
	if ctx.TaskConfigs != nil {
		if taskConfig, exists := ctx.TaskConfigs[taskID]; exists {
			cb.mergeTaskConfigWithoutParent(taskContext, taskConfig)
		}
	}
	return taskContext
}

func (cb *ContextBuilder) mergeTaskConfigWithoutParent(taskContext map[string]any, taskConfig *task.Config) {
	taskConfigMap, err := taskConfig.AsMap()
	if err != nil {
		return
	}
	for k, v := range taskConfigMap {
		if k != inputKey && k != outputKey && k != parentKey {
			taskContext[k] = v
		}
	}
}

func (cb *ContextBuilder) BuildCollectionContext(
	workflowState *workflow.State,
	taskConfig *task.Config,
) map[string]any {
	// Defensive null checks to prevent null pointer dereferences
	if workflowState == nil || taskConfig == nil {
		return make(map[string]any)
	}

	// Build full context similar to BuildContext but for collection tasks
	ctx := &NormalizationContext{
		WorkflowState: workflowState,
		TaskConfigs:   make(map[string]*task.Config),
	}
	cb.buildChildrenIndex(ctx)

	templateContext := map[string]any{
		workflowKey: cb.buildWorkflowContext(ctx),
		tasksKey:    cb.buildTasksContext(ctx),
	}

	// Add workflow input/output if available
	if workflowState.Input != nil {
		templateContext[inputKey] = *workflowState.Input
	}
	if workflowState.Output != nil {
		templateContext[outputKey] = *workflowState.Output
	}
	// Add task-specific context from 'with' parameter
	if taskConfig.With != nil {
		maps.Copy(templateContext, *taskConfig.With)
	}
	return templateContext
}
