package shared

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/dgraph-io/ristretto"
)

// NormalizationContext holds all data needed for normalization
type NormalizationContext struct {
	WorkflowState  *workflow.State
	WorkflowConfig *workflow.Config
	TaskConfig     *task.Config
	ParentTask     *task.Config
	ParentConfig   map[string]any          // Parent configuration properties
	TaskConfigs    map[string]*task.Config // Task configurations by ID
	CurrentInput   *core.Input
	MergedEnv      *core.EnvMap
	ChildrenIndex  map[string][]string // Maps parent task exec ID to child task IDs
	Variables      map[string]any
}

// GetVariables returns the variables map, creating it if necessary
func (nc *NormalizationContext) GetVariables() map[string]any {
	if nc.Variables == nil {
		nc.Variables = make(map[string]any)
	}
	return nc.Variables
}

// ContextBuilder builds normalization contexts with workflow and task data
type ContextBuilder struct {
	// Ristretto cache for parent contexts to prevent redundant building
	parentContextCache   *ristretto.Cache[string, map[string]any]
	VariableBuilder      *VariableBuilder
	ChildrenIndexBuilder *ChildrenIndexBuilder
	TaskOutputBuilder    TaskOutputBuilder
	ConfigMerger         *ConfigMerger
}

// NewContextBuilder creates a new context builder
func NewContextBuilder() (*ContextBuilder, error) {
	// Create Ristretto cache with proper configuration
	cache, err := ristretto.NewCache(&ristretto.Config[string, map[string]any]{
		NumCounters: 1000, // 10x expected unique parent contexts (100)
		MaxCost:     100,  // Max 100 parent contexts cached
		BufferItems: 64,   // Recommended buffer size
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create parent context cache: %w", err)
	}
	return &ContextBuilder{
		parentContextCache:   cache,
		VariableBuilder:      NewVariableBuilder(),
		ChildrenIndexBuilder: NewChildrenIndexBuilder(),
		TaskOutputBuilder:    NewTaskOutputBuilder(),
		ConfigMerger:         NewConfigMerger(),
	}, nil
}

// BuildContext creates a normalization context from workflow and task data
func (cb *ContextBuilder) BuildContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *NormalizationContext {
	nc := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfig:     taskConfig,
		TaskConfigs:    make(map[string]*task.Config),
	}

	// Build children index using dedicated builder
	nc.ChildrenIndex = cb.ChildrenIndexBuilder.BuildChildrenIndex(workflowState)

	// Build variables using dedicated builder
	vars := cb.VariableBuilder.BuildBaseVariables(workflowState, workflowConfig, taskConfig)

	// Add workflow output as tasks for backward compatibility
	if workflowState != nil && workflowState.Tasks != nil {
		tasksMap := make(map[string]any)
		for taskID, taskState := range workflowState.Tasks {
			tasksMap[taskID] = cb.buildSingleTaskContext(taskID, taskState, nc)
		}
		cb.VariableBuilder.AddTasksToVariables(vars, workflowState, tasksMap)
	}

	// Add current input if present
	cb.VariableBuilder.AddCurrentInputToVariables(vars, nc.CurrentInput)

	nc.Variables = vars
	return nc
}

// buildSingleTaskContext builds context for a single task
func (cb *ContextBuilder) buildSingleTaskContext(
	taskID string,
	taskState *task.State,
	ctx *NormalizationContext,
) map[string]any {
	if taskState == nil {
		return map[string]any{IDKey: taskID}
	}

	taskContext := map[string]any{
		IDKey:     taskID,
		InputKey:  taskState.Input,
		StatusKey: taskState.Status,
	}
	if taskState.Error != nil {
		taskContext[ErrorKey] = taskState.Error
	}
	taskContext[OutputKey] = cb.TaskOutputBuilder.BuildTaskOutput(
		taskState,
		ctx.WorkflowState.Tasks,
		ctx.ChildrenIndex,
		0,
	)
	if taskState.CanHaveChildren() && ctx.ChildrenIndex != nil {
		taskContext[ChildrenKey] = cb.ChildrenIndexBuilder.BuildChildrenContext(
			taskState,
			ctx.WorkflowState,
			ctx.ChildrenIndex,
			ctx.TaskConfigs,
			cb.TaskOutputBuilder,
			0,
		)
	}
	cb.ConfigMerger.MergeTaskConfigIfExists(taskContext, taskID, ctx.TaskConfigs)
	return taskContext
}

// BuildSubTaskContext creates a specialized context for sub-tasks within parent tasks
func (cb *ContextBuilder) BuildSubTaskContext(
	ctx *NormalizationContext,
	parentTask *task.Config,
	parentState *task.State,
) *NormalizationContext {
	// Clone the base context
	nc := &NormalizationContext{
		WorkflowState:  ctx.WorkflowState,
		WorkflowConfig: ctx.WorkflowConfig,
		ParentTask:     parentTask,
		Variables:      make(map[string]any),
		TaskConfigs:    ctx.TaskConfigs,
	}
	// Copy base variables using variable builder
	nc.Variables = cb.VariableBuilder.CopyVariables(ctx.Variables)

	// Use recursive parent building with caching
	if parentTask != nil {
		cb.VariableBuilder.AddParentToVariables(nc.Variables, cb.BuildParentContext(ctx, parentTask, 0))
	}
	// Add current task state if available
	if parentState != nil {
		currentMap := map[string]any{
			IDKey:     parentState.TaskID,
			InputKey:  parentState.Input,
			OutputKey: parentState.Output,
			StatusKey: parentState.Status,
		}
		if parentState.Error != nil {
			currentMap[ErrorKey] = parentState.Error
		}
		nc.Variables["current"] = currentMap
	}
	return nc
}

// BuildNormalizationSubTaskContext creates a context for sub-tasks during normalization
// This is used when we don't have runtime state yet
func (cb *ContextBuilder) BuildNormalizationSubTaskContext(
	parentCtx *NormalizationContext,
	parentTask *task.Config,
	subTask *task.Config,
) *NormalizationContext {
	// Clone the base context
	nc := &NormalizationContext{
		WorkflowState:  parentCtx.WorkflowState,
		WorkflowConfig: parentCtx.WorkflowConfig,
		TaskConfig:     subTask,
		ParentTask:     parentTask,
		Variables:      make(map[string]any),
		TaskConfigs:    parentCtx.TaskConfigs,
		CurrentInput:   parentCtx.CurrentInput,
		ChildrenIndex:  parentCtx.ChildrenIndex,
	}
	// Copy parent variables using variable builder
	nc.Variables = cb.VariableBuilder.CopyVariables(parentCtx.Variables)

	// Update task data for sub-task
	nc.Variables["task"] = map[string]any{
		IDKey:     subTask.ID,
		TypeKey:   subTask.Type,
		ActionKey: subTask.Action,
		WithKey:   subTask.With,
		EnvKey:    subTask.Env,
	}
	// Use recursive parent building with caching
	if parentTask != nil {
		cb.VariableBuilder.AddParentToVariables(nc.Variables, cb.BuildParentContext(parentCtx, parentTask, 0))
	}
	return nc
}

// BuildParentContext recursively builds parent context chain with caching
func (cb *ContextBuilder) BuildParentContext(
	ctx *NormalizationContext,
	taskConfig *task.Config,
	depth int,
) map[string]any {
	const maxParentDepth = 10 // Prevent infinite recursion
	if taskConfig == nil || depth >= maxParentDepth {
		return nil
	}
	// Create cache key using workflow ID and task ID for uniqueness
	cacheKey := cb.buildCacheKey(ctx, taskConfig)
	// Check cache first
	if cached, found := cb.parentContextCache.Get(cacheKey); found {
		return cached
	}
	// Build parent context map
	parentMap := map[string]any{
		IDKey:     taskConfig.ID,
		TypeKey:   taskConfig.Type,
		ActionKey: taskConfig.Action,
		EnvKey:    taskConfig.Env,
	}
	// Add With field as map
	if taskConfig.With != nil {
		// core.Input is likely map[string]any, so dereference it
		parentMap[WithKey] = *taskConfig.With
	} else {
		parentMap[WithKey] = taskConfig.With
	}
	// Add runtime state data if available
	if ctx.WorkflowState != nil && ctx.WorkflowState.Tasks != nil {
		if state, exists := ctx.WorkflowState.Tasks[taskConfig.ID]; exists {
			parentMap[InputKey] = state.Input
			parentMap[OutputKey] = state.Output
			parentMap[StatusKey] = state.Status
			if state.Error != nil {
				parentMap[ErrorKey] = state.Error
			}
		}
	}
	// Find and recursively add grandparent
	grandParentTask := cb.findParentTask(ctx, taskConfig)
	if grandParentTask != nil {
		parentMap[ParentKey] = cb.BuildParentContext(ctx, grandParentTask, depth+1)
	}
	// Store in cache with cost of 1
	cb.parentContextCache.Set(cacheKey, parentMap, 1)
	cb.parentContextCache.Wait()
	return parentMap
}

// buildCacheKey creates a unique cache key for a task in a workflow
func (cb *ContextBuilder) buildCacheKey(ctx *NormalizationContext, taskConfig *task.Config) string {
	if ctx.WorkflowState != nil {
		return fmt.Sprintf("%s:%s", ctx.WorkflowState.WorkflowID, taskConfig.ID)
	}
	// Fallback to just task ID if no workflow state
	return taskConfig.ID
}

// findParentTask searches for the parent of a given task
func (cb *ContextBuilder) findParentTask(
	ctx *NormalizationContext,
	childTask *task.Config,
) *task.Config {
	// First check if we have runtime state to find parent
	if ctx.WorkflowState != nil && ctx.WorkflowState.Tasks != nil {
		if childState, exists := ctx.WorkflowState.Tasks[childTask.ID]; exists {
			if childState.ParentStateID != nil {
				// Find parent task config by exec ID
				for _, taskState := range ctx.WorkflowState.Tasks {
					if taskState.TaskExecID == *childState.ParentStateID {
						// Found parent state, now find its config
						if ctx.TaskConfigs != nil {
							if parentConfig, ok := ctx.TaskConfigs[taskState.TaskID]; ok {
								return parentConfig
							}
						}
						// Try to find in workflow config
						if ctx.WorkflowConfig != nil {
							for i := range ctx.WorkflowConfig.Tasks {
								if ctx.WorkflowConfig.Tasks[i].ID == taskState.TaskID {
									return &ctx.WorkflowConfig.Tasks[i]
								}
							}
						}
						break
					}
				}
			}
		}
	}
	// Fallback: search through workflow config structure
	if ctx.WorkflowConfig != nil {
		return cb.searchParentInWorkflow(ctx.WorkflowConfig, childTask.ID)
	}
	return nil
}

// searchParentInWorkflow recursively searches for a task's parent in workflow config
func (cb *ContextBuilder) searchParentInWorkflow(
	workflow *workflow.Config,
	childTaskID string,
) *task.Config {
	// Search through all tasks in workflow
	for i := range workflow.Tasks {
		task := &workflow.Tasks[i]
		if cb.taskContainsChild(task, childTaskID) {
			return task
		}
	}
	return nil
}

// taskContainsChild checks if a task contains a specific child task
func (cb *ContextBuilder) taskContainsChild(
	parentTask *task.Config,
	childTaskID string,
) bool {
	// Check for different task types that can have children
	switch parentTask.Type {
	case task.TaskTypeParallel, task.TaskTypeComposite:
		if parentTask.Tasks != nil {
			for i := range parentTask.Tasks {
				subTask := &parentTask.Tasks[i]
				if subTask.ID == childTaskID {
					return true
				}
				// Recursively check nested tasks
				if cb.taskContainsChild(subTask, childTaskID) {
					return true
				}
			}
		}
	case task.TaskTypeCollection:
		// Collection tasks use Task field for the template
		if parentTask.Task != nil {
			if parentTask.Task.ID == childTaskID {
				return true
			}
			// Recursively check if collection task template contains the child
			return cb.taskContainsChild(parentTask.Task, childTaskID)
		}
	}
	return false
}

// ClearCache clears the parent context cache - should be called at workflow start
func (cb *ContextBuilder) ClearCache() {
	cb.parentContextCache.Clear()
}

// AddTaskState adds runtime task state data to the context variables.
// This includes execution IDs, status, output, and error information.
func (cb *ContextBuilder) AddTaskState(ctx *NormalizationContext, taskState *task.State) {
	if taskState == nil {
		return
	}

	vars := ctx.GetVariables()
	vars["state"] = map[string]any{
		IDKey:       taskState.TaskID,
		"exec_id":   taskState.TaskExecID,
		StatusKey:   taskState.Status,
		OutputKey:   taskState.Output,
		ErrorKey:    taskState.Error,
		"parent_id": taskState.ParentStateID,
	}
}

// AddCollectionData adds collection-specific iteration data to the context.
// This is used when processing collection tasks to make the current item and index available.
func (cb *ContextBuilder) AddCollectionData(ctx *NormalizationContext, item any, index int) {
	vars := ctx.GetVariables()
	vars[ItemKey] = item
	vars[IndexKey] = index
}

// MergeVariables merges additional variables into the normalization context.
// Existing variables with the same keys will be overwritten.
func (cb *ContextBuilder) MergeVariables(ctx *NormalizationContext, additionalVars map[string]any) {
	vars := ctx.GetVariables()
	for k, v := range additionalVars {
		vars[k] = v
	}
}

// BuildCollectionContext builds context for collection tasks
func (cb *ContextBuilder) BuildCollectionContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) map[string]any {
	// Defensive null checks to prevent null pointer dereferences
	if workflowState == nil || taskConfig == nil {
		return make(map[string]any)
	}

	// Build full context similar to BuildContext but for collection tasks
	ctx := &NormalizationContext{
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		TaskConfigs:    make(map[string]*task.Config),
	}

	// Populate TaskConfigs from workflowConfig if available
	if workflowConfig != nil && workflowConfig.Tasks != nil {
		for i := range workflowConfig.Tasks {
			tc := &workflowConfig.Tasks[i]
			ctx.TaskConfigs[tc.ID] = tc
		}
	}

	ctx.ChildrenIndex = cb.ChildrenIndexBuilder.BuildChildrenIndex(workflowState)

	templateContext := map[string]any{
		WorkflowKey: cb.buildWorkflowContext(ctx),
		TasksKey:    cb.buildTasksContext(ctx),
	}

	// Add workflow input/output if available
	if workflowState.Input != nil {
		templateContext[InputKey] = *workflowState.Input
	}
	if workflowState.Output != nil {
		templateContext[OutputKey] = *workflowState.Output
	}
	// Add task-specific context from 'with' parameter
	if taskConfig.With != nil {
		for k, v := range *taskConfig.With {
			templateContext[k] = v
		}
	}
	return templateContext
}

// buildWorkflowContext builds workflow context map
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
	// Overwrite/add runtime properties
	workflowContext[IDKey] = ctx.WorkflowState.WorkflowID
	workflowContext[InputKey] = ctx.WorkflowState.Input
	workflowContext[OutputKey] = ctx.WorkflowState.Output
	return workflowContext
}

// buildTasksContext builds tasks context map
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

// BuildTemplateContext returns the template context for normalization
func (nc *NormalizationContext) BuildTemplateContext() map[string]any {
	if nc.Variables == nil {
		return make(map[string]any)
	}
	return nc.Variables
}
