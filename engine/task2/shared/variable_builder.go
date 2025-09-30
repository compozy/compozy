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

// dereferenceInput safely dereferences the workflow input pointer
// This is critical for template resolution of memory keys like {{.workflow.input.user_id}}
func (vb *VariableBuilder) dereferenceInput(input *core.Input) any {
	if input == nil {
		return nil
	}
	// Dereference the pointer to expose the underlying map
	// This allows templates to access nested fields like .workflow.input.user_id
	return *input
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
		workflowData := map[string]any{
			"id":     workflowState.WorkflowID,
			"input":  vb.dereferenceInput(workflowState.Input),
			"output": workflowState.Output,
			"status": workflowState.Status,
		}
		// NOTE: We intentionally do NOT include workflowConfig here during normalization
		// to prevent premature template evaluation of task references.
		// The config is available through other means when needed at runtime.
		vars["workflow"] = workflowData
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
		dereferenced := vb.dereferenceInput(currentInput)
		vars["input"] = dereferenced
		vars["with"] = dereferenced
		// Also add item and index at top level for collection tasks
		if item, exists := (*currentInput)[FieldItem]; exists {
			vars[FieldItem] = item
		} else if collectionItem, exists := (*currentInput)[FieldCollectionItem]; exists {
			// Fallback: if "item" not found, check for "_collection_item"
			vars[FieldItem] = collectionItem
		}
		if index, exists := (*currentInput)[FieldIndex]; exists {
			vars[FieldIndex] = index
		} else if collectionIndex, exists := (*currentInput)[FieldCollectionIndex]; exists {
			// Fallback: if "index" not found, check for "_collection_index"
			vars[FieldIndex] = collectionIndex
		}
		// Handle collection-specific fields
		if collectionItem, exists := (*currentInput)[FieldCollectionItem]; exists {
			// Add custom item variable name if specified
			if itemVar, exists := (*currentInput)[FieldCollectionItemVar]; exists {
				if varName, ok := itemVar.(string); ok && varName != "" {
					vars[varName] = collectionItem
				}
			}
		}
		if collectionIndex, exists := (*currentInput)[FieldCollectionIndex]; exists {
			// Add custom index variable name if specified
			if indexVar, exists := (*currentInput)[FieldCollectionIndexVar]; exists {
				if varName, ok := indexVar.(string); ok && varName != "" {
					vars[varName] = collectionIndex
				}
			}
		}
	}
}

// CopyVariables creates a copy of variables map
func (vb *VariableBuilder) CopyVariables(source map[string]any) (map[string]any, error) {
	if source == nil {
		return make(map[string]any), nil
	}

	vars, err := core.DeepCopy(source)
	if err != nil {
		return nil, err
	}
	return vars, nil
}

// AddParentToVariables adds parent context to variables
func (vb *VariableBuilder) AddParentToVariables(vars map[string]any, parentContext map[string]any) {
	if parentContext != nil {
		vars["parent"] = parentContext
	}
}

// AddProjectToVariables adds project information to variables for memory operations
func (vb *VariableBuilder) AddProjectToVariables(vars map[string]any, projectName string) {
	if projectName != "" {
		vars["project"] = map[string]any{
			"id":   projectName,
			"name": projectName,
		}
	}
}
