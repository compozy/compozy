package shared

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// VariableBuilder is responsible for building template variables for normalization contexts
type VariableBuilder struct{}

// NewVariableBuilder creates a new variable builder
func NewVariableBuilder() *VariableBuilder {
	return &VariableBuilder{}
}

// BuildBaseVariables builds the base variables map from workflow and task data
func (vb *VariableBuilder) BuildBaseVariables(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) map[string]any {
	vars := make(map[string]any)

	// Add workflow data
	if workflowState != nil {
		vars["workflow"] = map[string]any{
			"id":     workflowState.WorkflowID,
			"input":  workflowState.Input,
			"output": workflowState.Output,
			"status": workflowState.Status,
			"config": workflowConfig,
		}
	}

	// Add task data
	if taskConfig != nil {
		vars["task"] = map[string]any{
			"id":     taskConfig.ID,
			"type":   taskConfig.Type,
			"action": taskConfig.Action,
			"with":   taskConfig.With,
			"env":    taskConfig.Env,
		}
	}

	// Add env from workflow config
	if workflowConfig != nil && workflowConfig.Opts.Env != nil {
		vars["env"] = workflowConfig.Opts.Env
	}

	return vars
}

// AddTasksToVariables adds tasks data to variables for backward compatibility
func (vb *VariableBuilder) AddTasksToVariables(
	vars map[string]any,
	workflowState *workflow.State,
	tasksMap map[string]any,
) {
	if workflowState != nil && workflowState.Tasks != nil && tasksMap != nil {
		vars["tasks"] = tasksMap
	}
}

// AddCurrentInputToVariables adds current input data to variables
func (vb *VariableBuilder) AddCurrentInputToVariables(vars map[string]any, currentInput *core.Input) {
	if currentInput != nil {
		vars["input"] = currentInput
		// Also add item and index at top level for collection tasks
		if item, exists := (*currentInput)["item"]; exists {
			vars["item"] = item
		}
		if index, exists := (*currentInput)["index"]; exists {
			vars["index"] = index
		}
	}
}

// CopyVariables creates a copy of variables map
func (vb *VariableBuilder) CopyVariables(source map[string]any) map[string]any {
	if source == nil {
		return make(map[string]any)
	}

	vars := make(map[string]any)
	for k, v := range source {
		vars[k] = v
	}
	return vars
}

// AddParentToVariables adds parent context to variables
func (vb *VariableBuilder) AddParentToVariables(vars map[string]any, parentContext map[string]any) {
	if parentContext != nil {
		vars["parent"] = parentContext
	}
}
