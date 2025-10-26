package shared_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestBuildChildrenContext_EdgeCases(t *testing.T) {
	t.Run("Should handle task with no children", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		taskState := &task.State{
			TaskID:        "parent-task",
			TaskExecID:    core.MustNewID(),
			Status:        core.StatusRunning,
			ExecutionType: task.ExecutionBasic,
		}
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"parent-task": taskState,
			},
		}
		childrenIndex := builder.BuildChildrenIndex(workflowState)
		taskConfigs := map[string]*task.Config{
			"parent-task": {
				BaseConfig: task.BaseConfig{
					ID:   "parent-task",
					Type: task.TaskTypeBasic,
				},
			},
		}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act
		result := builder.BuildChildrenContext(
			t.Context(),
			taskState,
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		assert.Empty(t, result)
	})

	t.Run("Should handle missing children states", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		parentExecID := core.MustNewID()
		taskState := &task.State{
			TaskID:        "parent-task",
			TaskExecID:    parentExecID,
			Status:        core.StatusRunning,
			ExecutionType: task.ExecutionComposite,
		}
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"parent-task": taskState,
			},
		}
		// Manually create children index with orphaned references
		childrenIndex := map[string][]string{
			parentExecID.String(): {"missing-child-1", "missing-child-2"},
		}
		taskConfigs := map[string]*task.Config{
			"parent-task": {
				BaseConfig: task.BaseConfig{
					ID:   "parent-task",
					Type: task.TaskTypeComposite,
				},
			},
		}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act
		result := builder.BuildChildrenContext(
			t.Context(),
			taskState,
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		assert.Empty(t, result, "Should return empty map when children states are missing")
	})

	t.Run("Should handle children with missing configs", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		parentExecID := core.MustNewID()
		childExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"parent-task": {
					TaskID:        "parent-task",
					TaskExecID:    parentExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionComposite,
				},
				"child-task": {
					TaskID:        "child-task",
					TaskExecID:    childExecID,
					Status:        core.StatusSuccess,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parentExecID,
					Output:        &core.Output{"result": "success"},
				},
			},
		}
		childrenIndex := builder.BuildChildrenIndex(workflowState)
		// No task configs provided
		taskConfigs := map[string]*task.Config{}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act
		result := builder.BuildChildrenContext(
			t.Context(),
			workflowState.Tasks["parent-task"],
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		require.Len(t, result, 1)
		childCtx, ok := result["child-task"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "child-task", childCtx[shared.IDKey])
		assert.Equal(t, core.StatusSuccess, childCtx[shared.StatusKey])
		assert.NotNil(t, childCtx[shared.OutputKey])
	})

	t.Run("Should handle maximum depth correctly", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		taskConfigs := make(map[string]*task.Config)
		taskStates := make(map[string]*task.State)
		// Create a deep hierarchy of tasks
		var previousExecID *core.ID
		for i := range 20 {
			taskID := string(rune('A'+i)) + "-task"
			execID := core.MustNewID()
			taskConfigs[taskID] = &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   taskID,
					Type: task.TaskTypeComposite,
				},
			}
			taskStates[taskID] = &task.State{
				TaskID:        taskID,
				TaskExecID:    execID,
				Status:        core.StatusRunning,
				ExecutionType: task.ExecutionComposite,
				ParentStateID: previousExecID,
			}
			previousExecID = &execID
		}
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks:          taskStates,
		}
		childrenIndex := builder.BuildChildrenIndex(workflowState)
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act - Get children for the root task
		rootTask := taskStates["A-task"]
		result := builder.BuildChildrenContext(
			t.Context(),
			rootTask,
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		// Count the depth of children hierarchy
		depth := 0
		current := result
		for len(current) > 0 && depth < 25 { // Safety limit
			depth++
			// Get first child and check its children
			for _, child := range current {
				if childMap, ok := child.(map[string]any); ok {
					if children, ok := childMap[shared.ChildrenKey].(map[string]any); ok {
						current = children
						break
					}
				}
				current = nil
				break
			}
		}
		// Should respect max depth limit
		limits := shared.GetGlobalConfigLimits(t.Context())
		assert.LessOrEqual(t, depth, limits.MaxChildrenDepth, "Should not exceed max children depth")
	})

	t.Run("Should handle collection children with multiple instances", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		collectionExecID := core.MustNewID()
		child1ExecID := core.MustNewID()
		child2ExecID := core.MustNewID()
		child3ExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"collection-task": {
					TaskID:        "collection-task",
					TaskExecID:    collectionExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionCollection,
				},
				"child-task-0": {
					TaskID:        "child-task",
					TaskExecID:    child1ExecID,
					Status:        core.StatusSuccess,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &collectionExecID,
					Input:         &core.Input{"item": "item1", "index": 0},
					Output:        &core.Output{"result": "processed-item1"},
				},
				"child-task-1": {
					TaskID:        "child-task",
					TaskExecID:    child2ExecID,
					Status:        core.StatusSuccess,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &collectionExecID,
					Input:         &core.Input{"item": "item2", "index": 1},
					Output:        &core.Output{"result": "processed-item2"},
				},
				"child-task-2": {
					TaskID:        "child-task",
					TaskExecID:    child3ExecID,
					Status:        core.StatusFailed,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &collectionExecID,
					Input:         &core.Input{"item": "item3", "index": 2},
					Error:         &core.Error{Message: "processing failed"},
				},
			},
		}
		childrenIndex := builder.BuildChildrenIndex(workflowState)
		taskConfigs := map[string]*task.Config{
			"collection-task": {
				BaseConfig: task.BaseConfig{
					ID:   "collection-task",
					Type: task.TaskTypeCollection,
				},
			},
			"child-task": {
				BaseConfig: task.BaseConfig{
					ID:   "child-task",
					Type: task.TaskTypeBasic,
				},
			},
		}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act
		result := builder.BuildChildrenContext(
			t.Context(),
			workflowState.Tasks["collection-task"],
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		require.Len(t, result, 3)
		// Check each child instance
		child0, ok := result["child-task-0"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, core.StatusSuccess, child0[shared.StatusKey])
		child1, ok := result["child-task-1"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, core.StatusSuccess, child1[shared.StatusKey])
		child2, ok := result["child-task-2"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, core.StatusFailed, child2[shared.StatusKey])
		assert.NotNil(t, child2[shared.ErrorKey])
	})

	t.Run("Should handle parallel children with different execution states", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		parallelExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"parallel-task": {
					TaskID:        "parallel-task",
					TaskExecID:    parallelExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionParallel,
				},
				"child-1": {
					TaskID:        "child-1",
					TaskExecID:    core.MustNewID(),
					Status:        core.StatusSuccess,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parallelExecID,
					Output:        &core.Output{"result": "done"},
				},
				"child-2": {
					TaskID:        "child-2",
					TaskExecID:    core.MustNewID(),
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parallelExecID,
				},
				"child-3": {
					TaskID:        "child-3",
					TaskExecID:    core.MustNewID(),
					Status:        core.StatusPending,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parallelExecID,
				},
			},
		}
		childrenIndex := builder.BuildChildrenIndex(workflowState)
		taskConfigs := map[string]*task.Config{}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act
		result := builder.BuildChildrenContext(
			t.Context(),
			workflowState.Tasks["parallel-task"],
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		require.Len(t, result, 3)
		// Verify each child's status
		statuses := make(map[string]int)
		for _, child := range result {
			if childMap, ok := child.(map[string]any); ok {
				if status := childMap[shared.StatusKey]; status != nil {
					statuses[fmt.Sprintf("%v", status)]++
				}
			}
		}
		assert.Equal(t, 1, statuses[string(core.StatusSuccess)])
		assert.Equal(t, 1, statuses[string(core.StatusRunning)])
		assert.Equal(t, 1, statuses[string(core.StatusPending)])
	})

	t.Run("Should handle circular reference in children", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		taskAExecID := core.MustNewID()
		taskBExecID := core.MustNewID()
		taskCExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"task-A": {
					TaskID:        "task-A",
					TaskExecID:    taskAExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionComposite,
					ParentStateID: &taskCExecID, // Circular: C -> A
				},
				"task-B": {
					TaskID:        "task-B",
					TaskExecID:    taskBExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionComposite,
					ParentStateID: &taskAExecID, // A -> B
				},
				"task-C": {
					TaskID:        "task-C",
					TaskExecID:    taskCExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionComposite,
					ParentStateID: &taskBExecID, // B -> C
				},
			},
		}
		childrenIndex := builder.BuildChildrenIndex(workflowState)
		taskConfigs := map[string]*task.Config{}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act
		result := builder.BuildChildrenContext(
			t.Context(),
			workflowState.Tasks["task-A"],
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		// Should handle circular reference gracefully
		require.NotNil(t, result)
		// The result should contain task-B
		taskB, ok := result["task-B"].(map[string]any)
		assert.True(t, ok)
		// Task B should have children (task-C)
		if ok && taskB[shared.ChildrenKey] != nil {
			children, ok := taskB[shared.ChildrenKey].(map[string]any)
			assert.True(t, ok)
			// Task C should have children that detect circular reference
			if ok {
				taskC, hasTaskC := children["task-C"].(map[string]any)
				if hasTaskC && taskC[shared.ChildrenKey] != nil {
					grandChildren, ok := taskC[shared.ChildrenKey].(map[string]any)
					assert.True(t, ok)
					// The circular reference should be detected here
					if ok {
						// Task-A should be present but its children should have the error
						taskA, hasTaskA := grandChildren["task-A"].(map[string]any)
						assert.True(t, hasTaskA)
						if hasTaskA {
							// Check task-A's children for circular reference error
							if taskAChildren, ok := taskA[shared.ChildrenKey].(map[string]any); ok {
								assert.Contains(t, taskAChildren, "error")
								assert.Equal(t, "circular reference detected in children chain", taskAChildren["error"])
							}
						}
					}
				}
			}
		}
	})

	t.Run("Should handle nil workflow state", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		taskState := &task.State{
			TaskID:        "test-task",
			TaskExecID:    core.MustNewID(),
			Status:        core.StatusRunning,
			ExecutionType: task.ExecutionComposite,
		}
		childrenIndex := map[string][]string{}
		taskConfigs := map[string]*task.Config{}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act
		result := builder.BuildChildrenContext(
			t.Context(),
			taskState,
			nil,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		// Assert
		assert.Empty(t, result)
	})

	t.Run("Should handle nil task state", func(t *testing.T) {
		// Arrange
		builder := shared.NewChildrenIndexBuilder()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks:          map[string]*task.State{},
		}
		childrenIndex := map[string][]string{}
		taskConfigs := map[string]*task.Config{}
		taskOutputBuilder := shared.NewTaskOutputBuilder(t.Context())
		// Act & Assert - Should not panic
		result := builder.BuildChildrenContext(
			t.Context(),
			nil,
			workflowState,
			childrenIndex,
			taskConfigs,
			taskOutputBuilder,
			0,
		)
		assert.Empty(t, result)
	})
}
