package shared

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
)

// TaskOutputBuilder is responsible for building task output structures
type TaskOutputBuilder interface {
	BuildTaskOutput(
		taskState *task.State,
		workflowTasks map[string]*task.State,
		childrenIndex map[string][]string,
		depth int,
	) any
	BuildEmptyOutput() core.Output
}

// DefaultTaskOutputBuilder implements TaskOutputBuilder
type DefaultTaskOutputBuilder struct {
	maxDepth int
}

// NewTaskOutputBuilder creates a new task output builder
func NewTaskOutputBuilder() TaskOutputBuilder {
	maxDepth := getMaxContextDepthFromConfig()
	return &DefaultTaskOutputBuilder{
		maxDepth: maxDepth,
	}
}

// NewTaskOutputBuilderWithContext creates a new task output builder using limits
// derived from the provided context when available. Falls back to the standard
// configuration loading when values are absent.
func NewTaskOutputBuilderWithContext(ctx context.Context) TaskOutputBuilder {
	if ctx == nil {
		return NewTaskOutputBuilder()
	}
	// Prefer values already attached to the context
	if cfg := config.FromContext(ctx); cfg != nil && cfg.Limits.MaxTaskContextDepth > 0 {
		return &DefaultTaskOutputBuilder{maxDepth: cfg.Limits.MaxTaskContextDepth}
	}
	// As a fallback, reuse the existing logic
	return NewTaskOutputBuilder()
}

// getMaxContextDepthFromConfig gets the max context depth from config
// with a default fallback of 10
func getMaxContextDepthFromConfig() int {
	const defaultMaxDepth = 10
	// Load configuration from environment
	service := config.NewService()
	ctx := context.Background()
	appConfig, err := service.Load(ctx)
	if err == nil && appConfig.Limits.MaxTaskContextDepth > 0 {
		return appConfig.Limits.MaxTaskContextDepth
	}
	return defaultMaxDepth
}

// BuildTaskOutput builds task output recursively
func (tob *DefaultTaskOutputBuilder) BuildTaskOutput(
	taskState *task.State,
	workflowTasks map[string]*task.State,
	childrenIndex map[string][]string,
	depth int,
) any {
	// Prevent unbounded recursion
	if depth >= tob.maxDepth || taskState == nil {
		return nil
	}
	if taskState.CanHaveChildren() {
		// For parent tasks, build nested output structure with child task outputs
		nestedOutput := make(map[string]any)
		// Include the parent's own output first (if any)
		if taskState.Output != nil {
			nestedOutput["output"] = *taskState.Output
		}
		// Use pre-built children index for O(1) lookup
		if childrenIndex != nil && workflowTasks != nil {
			parentTaskExecID := string(taskState.TaskExecID)
			if childTaskIDs, exists := childrenIndex[parentTaskExecID]; exists {
				for _, childTaskID := range childTaskIDs {
					if childTaskState, exists := workflowTasks[childTaskID]; exists {
						// Add child task output to nested structure
						childOutput := make(map[string]any)
						childOutput["output"] = tob.BuildTaskOutput(
							childTaskState,
							workflowTasks,
							childrenIndex,
							depth+1,
						)
						childOutput["status"] = childTaskState.Status
						if childTaskState.Error != nil {
							childOutput["error"] = childTaskState.Error
						}
						nestedOutput[childTaskID] = childOutput
					}
				}
			}
		}
		return nestedOutput
	}
	if taskState.Output != nil && len(*taskState.Output) > 0 {
		return *taskState.Output
	}
	return core.Output{}
}

// BuildEmptyOutput returns an empty output
func (tob *DefaultTaskOutputBuilder) BuildEmptyOutput() core.Output {
	return core.Output{}
}
