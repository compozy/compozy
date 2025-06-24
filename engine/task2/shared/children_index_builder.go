package shared

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// ChildrenIndexBuilder is responsible for building parent-child relationship indexes
type ChildrenIndexBuilder struct{}

// NewChildrenIndexBuilder creates a new children index builder
func NewChildrenIndexBuilder() *ChildrenIndexBuilder {
	return &ChildrenIndexBuilder{}
}

// BuildChildrenIndex builds an index of parent-child relationships
func (cib *ChildrenIndexBuilder) BuildChildrenIndex(workflowState *workflow.State) map[string][]string {
	if workflowState == nil || workflowState.Tasks == nil {
		return make(map[string][]string)
	}
	childrenIndex := make(map[string][]string)
	for taskID, taskState := range workflowState.Tasks {
		if taskState.ParentStateID != nil {
			parentExecID := string(*taskState.ParentStateID)
			childrenIndex[parentExecID] = append(childrenIndex[parentExecID], taskID)
		}
	}
	return childrenIndex
}

// BuildChildrenContext builds a map containing all child task contexts for a parent task.
// It recursively builds context for each child task, including their status, output, and nested children if any.
// The depth parameter prevents unbounded recursion.
func (cib *ChildrenIndexBuilder) BuildChildrenContext(
	parentState *task.State,
	workflowState *workflow.State,
	childrenIndex map[string][]string,
	taskConfigs map[string]*task.Config,
	taskOutputBuilder TaskOutputBuilder,
	depth int,
) map[string]any {
	const maxContextDepth = 10
	if depth >= maxContextDepth {
		return make(map[string]any)
	}
	children := make(map[string]any)
	parentExecID := string(parentState.TaskExecID)
	if childTaskIDs, exists := childrenIndex[parentExecID]; exists {
		for _, childTaskID := range childTaskIDs {
			if childState, exists := workflowState.Tasks[childTaskID]; exists {
				children[childTaskID] = cib.buildChildContextWithoutParent(
					childTaskID,
					childState,
					workflowState,
					childrenIndex,
					taskConfigs,
					taskOutputBuilder,
					depth+1,
				)
			}
		}
	}
	return children
}

// buildChildContextWithoutParent builds child context without parent reference
func (cib *ChildrenIndexBuilder) buildChildContextWithoutParent(
	taskID string,
	taskState *task.State,
	workflowState *workflow.State,
	childrenIndex map[string][]string,
	taskConfigs map[string]*task.Config,
	taskOutputBuilder TaskOutputBuilder,
	depth int,
) map[string]any {
	taskContext := map[string]any{
		"id":     taskID,
		"input":  taskState.Input,
		"status": taskState.Status,
	}
	if taskState.Error != nil {
		taskContext["error"] = taskState.Error
	}
	// Ensure output key is always present for consistency
	if taskState.Output != nil {
		taskContext["output"] = *taskState.Output
	} else {
		taskContext["output"] = taskOutputBuilder.BuildEmptyOutput()
	}
	if taskState.CanHaveChildren() && childrenIndex != nil {
		taskContext["children"] = cib.BuildChildrenContext(
			taskState,
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			depth,
		)
	}
	if taskConfigs != nil {
		if taskConfig, exists := taskConfigs[taskID]; exists {
			if err := cib.mergeTaskConfigWithoutParent(taskContext, taskConfig); err != nil {
				// Log error but continue - best effort merge
				taskContext["_merge_error"] = err.Error()
			}
		}
	}
	return taskContext
}

// mergeTaskConfigWithoutParent merges task config without parent
func (cib *ChildrenIndexBuilder) mergeTaskConfigWithoutParent(
	taskContext map[string]any,
	taskConfig *task.Config,
) error {
	taskConfigMap, err := taskConfig.AsMap()
	if err != nil {
		return err
	}
	for k, v := range taskConfigMap {
		if k != "input" && k != "output" && k != "parent" {
			taskContext[k] = v
		}
	}
	return nil
}
