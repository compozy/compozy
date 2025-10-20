package shared

import (
	"context"

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
	keys := SortedMapKeys(workflowState.Tasks)
	for _, taskID := range keys {
		taskState := workflowState.Tasks[taskID]
		if taskState.ParentStateID != nil {
			parentExecID := string(*taskState.ParentStateID)
			childrenIndex[parentExecID] = append(childrenIndex[parentExecID], taskID)
		}
	}
	return childrenIndex
}

// BuildChildrenContext builds a map containing all child task contexts for a parent task.
// It recursively builds context for each child task, including their status, output, and nested children if any.
// The depth parameter and cycle detection prevent unbounded recursion.
func (cib *ChildrenIndexBuilder) BuildChildrenContext(
	ctx context.Context,
	parentState *task.State,
	workflowState *workflow.State,
	childrenIndex map[string][]string,
	taskConfigs map[string]*task.Config,
	taskOutputBuilder TaskOutputBuilder,
	depth int,
) map[string]any {
	return cib.buildChildrenContextWithVisited(
		ctx,
		parentState, workflowState, childrenIndex, taskConfigs,
		taskOutputBuilder, depth, make(map[string]bool),
	)
}

// buildChildrenContextWithVisited builds children context with cycle detection
func (cib *ChildrenIndexBuilder) buildChildrenContextWithVisited(
	ctx context.Context,
	parentState *task.State,
	workflowState *workflow.State,
	childrenIndex map[string][]string,
	taskConfigs map[string]*task.Config,
	taskOutputBuilder TaskOutputBuilder,
	depth int,
	visited map[string]bool,
) map[string]any {
	limits := GetGlobalConfigLimits(context.WithoutCancel(ctx))
	if depth >= limits.MaxChildrenDepth || parentState == nil {
		return make(map[string]any)
	}
	// Check for circular reference
	parentExecID := string(parentState.TaskExecID)
	if visited[parentExecID] {
		return map[string]any{
			"error": "circular reference detected in children chain",
		}
	}
	// Mark as visited
	visited[parentExecID] = true
	defer func() {
		delete(visited, parentExecID) // Clean up for other branches
	}()
	children := make(map[string]any)
	if childTaskIDs, exists := childrenIndex[parentExecID]; exists {
		for _, childTaskID := range childTaskIDs {
			if childState, exists := workflowState.Tasks[childTaskID]; exists {
				// Pass the original visited map directly since defer cleanup handles it
				children[childTaskID] = cib.buildChildContextWithoutParentVisited(
					ctx,
					childTaskID,
					childState,
					workflowState,
					childrenIndex,
					taskConfigs,
					taskOutputBuilder,
					depth+1,
					visited,
				)
			}
		}
	}
	return children
}

// buildChildContextWithoutParentVisited builds child context without parent reference with cycle detection
func (cib *ChildrenIndexBuilder) buildChildContextWithoutParentVisited(
	ctx context.Context,
	taskID string,
	taskState *task.State,
	workflowState *workflow.State,
	childrenIndex map[string][]string,
	taskConfigs map[string]*task.Config,
	taskOutputBuilder TaskOutputBuilder,
	depth int,
	visited map[string]bool,
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
		taskContext["children"] = cib.buildChildrenContextWithVisited(
			ctx,
			taskState,
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			depth,
			visited,
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
	keys := SortedMapKeys(taskConfigMap)
	for _, k := range keys {
		v := taskConfigMap[k]
		if k != "input" && k != OutputKey && k != "parent" {
			taskContext[k] = v
		}
	}
	return nil
}
