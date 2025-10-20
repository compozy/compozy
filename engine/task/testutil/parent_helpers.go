package testutil

import (
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
)

// buildParent creates a parent task state with the given strategy
func BuildParent(_ task.ParallelStrategy) *task.State {
	now := time.Now()
	parentID, _ := core.NewID()       //nolint:errcheck // Safe to ignore in test code
	workflowExecID, _ := core.NewID() //nolint:errcheck // Safe to ignore in test code
	return &task.State{
		Component:      core.ComponentTask,
		Status:         core.StatusPending,
		TaskID:         "parent-task",
		TaskExecID:     parentID,
		WorkflowID:     "test-workflow",
		WorkflowExecID: workflowExecID,
		ExecutionType:  task.ExecutionParallel,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// buildChild creates a child task state with the given parent ID and status
func BuildChild(parentID core.ID, status core.StatusType) *task.State {
	now := time.Now()
	childID, _ := core.NewID()        //nolint:errcheck // Safe to ignore in test code
	workflowExecID, _ := core.NewID() //nolint:errcheck // Safe to ignore in test code
	return &task.State{
		Component:      core.ComponentTask,
		Status:         status,
		TaskID:         "child-task-" + string(childID),
		TaskExecID:     childID,
		WorkflowID:     "test-workflow",
		WorkflowExecID: workflowExecID,
		ParentStateID:  &parentID,
		ExecutionType:  task.ExecutionBasic,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// BuildChildWithTaskID creates a child task state with a specific task ID
func BuildChildWithTaskID(parentID core.ID, status core.StatusType, taskID string) *task.State {
	child := BuildChild(parentID, status)
	child.TaskID = taskID
	return child
}
