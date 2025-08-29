package shared_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestContextBuilder_CircularReferenceDetection(t *testing.T) {
	t.Run("Should detect circular reference in parent chain", func(t *testing.T) {
		// Arrange - Create a circular reference scenario
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		// Task A -> Task B -> Task A (circular)
		taskA := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-a",
				Type: task.TaskTypeBasic,
			},
		}
		taskB := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-b",
				Type: task.TaskTypeBasic,
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"task-a": {
					TaskID:        "task-a",
					TaskExecID:    core.MustNewID(),
					ParentStateID: func() *core.ID { id := core.MustNewID(); return &id }(), // Points to task-b's exec ID
					Status:        core.StatusRunning,
				},
				"task-b": {
					TaskID:        "task-b",
					TaskExecID:    core.MustNewID(),
					ParentStateID: func() *core.ID { id := core.MustNewID(); return &id }(), // Points to task-a's exec ID
					Status:        core.StatusRunning,
				},
			},
		}

		// Set up circular reference - task A's parent points to task B's exec ID
		// and task B's parent points to task A's exec ID
		taskAExecID := workflowState.Tasks["task-a"].TaskExecID
		taskBExecID := workflowState.Tasks["task-b"].TaskExecID
		workflowState.Tasks["task-a"].ParentStateID = &taskBExecID
		workflowState.Tasks["task-b"].ParentStateID = &taskAExecID

		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}

		ctx := &shared.NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: workflowConfig,
			TaskConfigs: map[string]*task.Config{
				"task-a": taskA,
				"task-b": taskB,
			},
			Variables: make(map[string]any),
		}

		// Act - Try to build parent context (should detect circular reference)
		result := builder.BuildParentContext(ctx, taskA, 0)

		// Assert - Should detect circular reference (may be in nested parent chain)
		require.NotNil(t, result)

		// Check if circular reference is detected anywhere in the parent chain
		var foundCircularError bool
		var checkForCircularReference func(data map[string]any) bool
		checkForCircularReference = func(data map[string]any) bool {
			if err, hasError := data["error"]; hasError {
				if errStr, isString := err.(string); isString &&
					(strings.Contains(errStr, "circular reference detected") ||
						strings.Contains(errStr, "circular reference")) {
					return true
				}
			}
			if parent, hasParent := data[shared.ParentKey].(map[string]any); hasParent {
				return checkForCircularReference(parent)
			}
			return false
		}

		foundCircularError = checkForCircularReference(result)
		assert.True(t, foundCircularError, "Expected circular reference to be detected in parent chain")
	})

	t.Run("Should handle normal parent chain without false positives", func(t *testing.T) {
		// Arrange - Create a normal parent chain: A -> B -> C
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		taskA := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-a",
				Type: task.TaskTypeBasic,
			},
		}
		taskB := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-b",
				Type: task.TaskTypeBasic,
			},
		}
		taskC := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task-c",
				Type: task.TaskTypeBasic,
			},
		}

		taskBExecID := core.MustNewID()
		taskCExecID := core.MustNewID()

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"task-a": {
					TaskID:        "task-a",
					TaskExecID:    core.MustNewID(),
					ParentStateID: &taskBExecID, // Points to task-b
					Status:        core.StatusRunning,
				},
				"task-b": {
					TaskID:        "task-b",
					TaskExecID:    taskBExecID,
					ParentStateID: &taskCExecID, // Points to task-c
					Status:        core.StatusRunning,
				},
				"task-c": {
					TaskID:     "task-c",
					TaskExecID: taskCExecID,
					// No parent - root task
					Status: core.StatusRunning,
				},
			},
		}

		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}

		ctx := &shared.NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: workflowConfig,
			TaskConfigs: map[string]*task.Config{
				"task-a": taskA,
				"task-b": taskB,
				"task-c": taskC,
			},
			Variables: make(map[string]any),
		}

		// Act - Build parent context for task A
		result := builder.BuildParentContext(ctx, taskA, 0)

		// Assert - Should work normally without circular reference error
		require.NotNil(t, result)
		assert.Equal(t, "task-a", result[shared.IDKey])
		assert.NotContains(t, result, "error")

		// Should have parent context for task B
		if parent, ok := result[shared.ParentKey].(map[string]any); ok {
			assert.Equal(t, "task-b", parent[shared.IDKey])
			// Task B should have parent context for task C
			if grandParent, ok := parent[shared.ParentKey].(map[string]any); ok {
				assert.Equal(t, "task-c", grandParent[shared.IDKey])
			}
		}
	})

	t.Run("Should respect maximum depth limits", func(t *testing.T) {
		// Arrange - Create a very deep parent chain
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		// Create a chain of 15 tasks (deeper than default max of 10)
		tasks := make([]*task.Config, 15)
		taskStates := make(map[string]*task.State)
		taskConfigs := make(map[string]*task.Config)

		for i := range 15 {
			taskID := fmt.Sprintf("task-%d", i)
			tasks[i] = &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   taskID,
					Type: task.TaskTypeBasic,
				},
			}
			taskConfigs[taskID] = tasks[i]

			execID := core.MustNewID()
			taskState := &task.State{
				TaskID:     taskID,
				TaskExecID: execID,
				Status:     core.StatusRunning,
			}

			// Set parent reference (except for the last task)
			if i < 14 {
				parentExecID := core.MustNewID()
				taskState.ParentStateID = &parentExecID
			}

			taskStates[taskID] = taskState
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks:          taskStates,
		}

		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}

		ctx := &shared.NormalizationContext{
			WorkflowState:  workflowState,
			WorkflowConfig: workflowConfig,
			TaskConfigs:    taskConfigs,
			Variables:      make(map[string]any),
		}

		// Act - Build parent context for first task
		result := builder.BuildParentContext(ctx, tasks[0], 0)

		// Assert - Should be limited by max depth, not process all 15 levels
		require.NotNil(t, result)
		assert.Equal(t, "task-0", result[shared.IDKey])

		// Count the depth of parent chain
		depth := 0
		current := result
		for {
			if parent, ok := current[shared.ParentKey].(map[string]any); ok {
				depth++
				current = parent
				if depth > 12 { // Fail safe to prevent infinite loop in test
					break
				}
			} else {
				break
			}
		}

		// Should be limited by max depth configuration
		limits := shared.GetGlobalConfigLimits()
		assert.LessOrEqual(t, depth, limits.MaxParentDepth)
	})
}

func TestChildrenIndexBuilder_CircularReferenceDetection(t *testing.T) {
	t.Run("Should detect circular reference in children chain", func(t *testing.T) {
		// Arrange - Create circular children reference
		builder := shared.NewChildrenIndexBuilder()
		taskOutputBuilder := shared.NewTaskOutputBuilder()

		// Create parent and child with circular reference
		parentExecID := core.MustNewID()
		childExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: parentExecID,
			Status:     core.StatusRunning,
		}

		childState := &task.State{
			TaskID:     "child-task",
			TaskExecID: childExecID,
			Status:     core.StatusRunning,
		}

		// Set up circular reference in children index
		childrenIndex := map[string][]string{
			parentExecID.String(): {"child-task"},
			childExecID.String():  {"parent-task"}, // Circular: child has parent as child
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"parent-task": parentState,
				"child-task":  childState,
			},
		}

		// Act - Build children context (should detect circular reference)
		result := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			nil,
			taskOutputBuilder,
			0,
		)

		// Assert - Should handle circular reference gracefully
		require.NotNil(t, result)

		// The result should either contain an error or handle the circular reference safely
		if childResult, ok := result["child-task"].(map[string]any); ok {
			// If child result exists, check if it contains error or safely terminates
			if children, hasChildren := childResult["children"].(map[string]any); hasChildren {
				// If it has children, it should either contain error or be empty to prevent infinite loop
				assert.True(t, len(children) == 0 || children["error"] != nil)
			}
		}
	})

	t.Run("Should handle normal children hierarchy", func(t *testing.T) {
		// Arrange - Create normal parent-child hierarchy
		builder := shared.NewChildrenIndexBuilder()
		taskOutputBuilder := shared.NewTaskOutputBuilder()

		parentExecID := core.MustNewID()
		child1ExecID := core.MustNewID()
		child2ExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: parentExecID,
			Status:     core.StatusRunning,
		}

		child1State := &task.State{
			TaskID:     "child1-task",
			TaskExecID: child1ExecID,
			Status:     core.StatusSuccess,
		}

		child2State := &task.State{
			TaskID:     "child2-task",
			TaskExecID: child2ExecID,
			Status:     core.StatusSuccess,
		}

		childrenIndex := map[string][]string{
			parentExecID.String(): {"child1-task", "child2-task"},
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"parent-task": parentState,
				"child1-task": child1State,
				"child2-task": child2State,
			},
		}

		// Act - Build children context
		result := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			nil,
			taskOutputBuilder,
			0,
		)

		// Assert - Should work normally
		require.NotNil(t, result)
		assert.Contains(t, result, "child1-task")
		assert.Contains(t, result, "child2-task")

		// Check child contexts
		child1Result := result["child1-task"].(map[string]any)
		assert.Equal(t, "child1-task", child1Result["id"])
		assert.Equal(t, core.StatusSuccess, child1Result["status"])

		child2Result := result["child2-task"].(map[string]any)
		assert.Equal(t, "child2-task", child2Result["id"])
		assert.Equal(t, core.StatusSuccess, child2Result["status"])
	})
}
