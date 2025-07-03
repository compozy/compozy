package shared

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// BaseContextBuilder provides common context building functionality
type BaseContextBuilder struct {
	VariableBuilder      *VariableBuilder
	ChildrenIndexBuilder *ChildrenIndexBuilder
	TaskOutputBuilder    TaskOutputBuilder
	ConfigMerger         *ConfigMerger
}

// NewBaseContextBuilder creates a new base context builder
func NewBaseContextBuilder() *BaseContextBuilder {
	return &BaseContextBuilder{
		VariableBuilder:      NewVariableBuilder(),
		ChildrenIndexBuilder: NewChildrenIndexBuilder(),
		TaskOutputBuilder:    NewTaskOutputBuilder(),
		ConfigMerger:         NewConfigMerger(),
	}
}

// BuildContext creates a normalization context from workflow and task data
func (b *BaseContextBuilder) BuildContext(
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
	// Build children index
	nc.ChildrenIndex = b.ChildrenIndexBuilder.BuildChildrenIndex(workflowState)
	// Build base variables
	vars := b.VariableBuilder.BuildBaseVariables(workflowState, workflowConfig, taskConfig)
	// Add workflow tasks for backward compatibility in deterministic order
	if workflowState != nil && workflowState.Tasks != nil {
		tasksMap := make(map[string]any)
		keys := SortedMapKeys(workflowState.Tasks)
		for _, taskID := range keys {
			taskState := workflowState.Tasks[taskID]
			tasksMap[taskID] = b.buildSingleTaskContext(taskID, taskState, nc)
		}
		b.VariableBuilder.AddTasksToVariables(vars, workflowState, tasksMap)
	}
	// Add current input if present
	b.VariableBuilder.AddCurrentInputToVariables(vars, nc.CurrentInput)
	nc.Variables = vars
	return nc
}

// EnrichContext adds additional data to an existing context
func (b *BaseContextBuilder) EnrichContext(ctx *NormalizationContext, taskState *task.State) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if taskState == nil {
		return nil
	}
	// Add task state data to variables
	if ctx.Variables == nil {
		ctx.Variables = make(map[string]any)
	}
	// Add task output to variables if available
	if taskState.Output != nil {
		if taskMap, ok := ctx.Variables["task"].(map[string]any); ok {
			taskMap["output"] = taskState.Output
		}
	}
	// Add task state status
	if taskMap, ok := ctx.Variables["task"].(map[string]any); ok {
		taskMap["status"] = taskState.Status
		taskMap["error"] = taskState.Error
	}
	return nil
}

// ValidateContext ensures the context has all required fields
func (b *BaseContextBuilder) ValidateContext(ctx *NormalizationContext) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if ctx.WorkflowState == nil {
		return fmt.Errorf("workflow state is required")
	}
	if ctx.WorkflowConfig == nil {
		return fmt.Errorf("workflow config is required")
	}
	if ctx.Variables == nil {
		return fmt.Errorf("variables map is required")
	}
	return nil
}

// buildSingleTaskContext builds context for a single task
func (b *BaseContextBuilder) buildSingleTaskContext(
	taskID string,
	taskState *task.State,
	nc *NormalizationContext,
) map[string]any {
	taskContext := map[string]any{
		"id":     taskID,
		"status": taskState.Status,
	}
	// Add execution type if available
	if taskState.ExecutionType != "" {
		taskContext["execution_type"] = taskState.ExecutionType
	}
	// Build task output using dedicated builder
	if taskState.Output != nil {
		// For single task context, we don't need full recursive output building
		taskContext["output"] = taskState.Output
	}
	// Add input if available
	if taskState.Input != nil {
		taskContext["input"] = taskState.Input
	}
	// Add error if available
	if taskState.Error != nil {
		taskContext["error"] = taskState.Error
	}
	// Add children if task can have children
	if taskState.CanHaveChildren() && nc != nil && nc.ChildrenIndex != nil {
		taskContext["children"] = b.ChildrenIndexBuilder.BuildChildrenContext(
			taskState,
			nc.WorkflowState,
			nc.ChildrenIndex,
			nc.TaskConfigs,
			b.TaskOutputBuilder,
			0, // Start at depth 0
		)
	}
	return taskContext
}

// AddCurrentInput sets the current input in the context
func (b *BaseContextBuilder) AddCurrentInput(ctx *NormalizationContext, input *core.Input) {
	if ctx == nil || input == nil {
		return
	}
	ctx.CurrentInput = input
	b.VariableBuilder.AddCurrentInputToVariables(ctx.Variables, input)
}

// AddParentTask sets the parent task in the context
func (b *BaseContextBuilder) AddParentTask(ctx *NormalizationContext, parentTask *task.Config) {
	if ctx == nil || parentTask == nil {
		return
	}
	ctx.ParentTask = parentTask
	// Add parent context to variables
	if parentConfig, err := parentTask.AsMap(); err == nil {
		b.VariableBuilder.AddParentToVariables(ctx.Variables, parentConfig)
	}
}
