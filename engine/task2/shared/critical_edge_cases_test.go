package shared_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestCriticalEdgeCases_ParentChildAccessPatterns(t *testing.T) {
	t.Run("Should handle parent access with missing parent state", func(t *testing.T) {
		// Arrange - Child task references non-existent parent
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		nonExistentParentID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"child-task": {
					TaskID:        "child-task",
					TaskExecID:    core.MustNewID(),
					ParentStateID: &nonExistentParentID, // References non-existent parent
					Status:        core.StatusRunning,
				},
			},
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeBasic,
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"child-task": taskConfig,
			},
			Variables: make(map[string]any),
		}

		// Act - Try to build parent context with missing parent
		result := builder.BuildParentContext(ctx, taskConfig, 0)

		// Assert - Should handle gracefully (nil or empty result)
		// This should not panic or cause issues
		if result != nil {
			assert.IsType(t, map[string]any{}, result)
		}
	})

	t.Run("Should handle children access with orphaned children", func(t *testing.T) {
		// Arrange - Children reference missing parent
		builder := shared.NewChildrenIndexBuilder()
		outputBuilder := shared.NewTaskOutputBuilder()

		parentExecID := core.MustNewID()
		childExecID := core.MustNewID()

		// Parent state missing, but children index still references it
		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"child-task": {
					TaskID:     "child-task",
					TaskExecID: childExecID,
					Status:     core.StatusRunning,
				},
				// Parent task is missing from workflow state
			},
		}

		// Children index still has reference to missing parent
		childrenIndex := map[string][]string{
			string(parentExecID): {"child-task"},
		}

		// Act - Build children context with missing parent state
		result := builder.BuildChildrenContext(
			&task.State{
				TaskID:     "missing-parent",
				TaskExecID: parentExecID,
				Status:     core.StatusRunning,
			},
			workflowState,
			childrenIndex,
			nil,
			outputBuilder,
			0,
		)

		// Assert - Should handle gracefully
		require.NotNil(t, result)
		// Should either be empty or contain proper child context
		if childResult, ok := result["child-task"]; ok {
			assert.IsType(t, map[string]any{}, childResult)
		}
	})

	t.Run("Should handle deeply nested parent-child chains", func(t *testing.T) {
		// Arrange - Create a deep parent-child chain (but within limits)
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		const chainDepth = 8 // Less than DefaultMaxParentDepth (10)

		taskConfigs := make(map[string]*task.Config, chainDepth)
		taskStates := make(map[string]*task.State, chainDepth)

		// Create chain: task-0 -> task-1 -> task-2 -> ... -> task-7
		for i := 0; i < chainDepth; i++ {
			taskID := fmt.Sprintf("task-%d", i)
			execID := core.MustNewID()

			taskConfigs[taskID] = &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   taskID,
					Type: task.TaskTypeBasic,
				},
			}

			state := &task.State{
				TaskID:     taskID,
				TaskExecID: execID,
				Status:     core.StatusRunning,
			}

			// Link to parent (except for the root task)
			if i > 0 {
				parentTaskID := fmt.Sprintf("task-%d", i-1)
				parentExecID := taskStates[parentTaskID].TaskExecID
				state.ParentStateID = &parentExecID
			}

			taskStates[taskID] = state
		}

		workflowState := &workflow.State{
			WorkflowID:     "deep-chain-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks:          taskStates,
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs:   taskConfigs,
			Variables:     make(map[string]any),
		}

		// Act - Build parent context for the deepest child
		deepestChild := taskConfigs["task-7"]
		result := builder.BuildParentContext(ctx, deepestChild, 0)

		// Assert - Should traverse the entire chain successfully
		require.NotNil(t, result)
		assert.Equal(t, "task-7", result[shared.IDKey])

		// Should have parent chain
		current := result
		for i := 6; i >= 0; i-- { // Traverse up the chain
			if parent, ok := current[shared.ParentKey].(map[string]any); ok {
				expectedParentID := fmt.Sprintf("task-%d", i)
				assert.Equal(t, expectedParentID, parent[shared.IDKey])
				current = parent
			} else if i > 0 { // Should have parent unless we're at root
				t.Errorf("Missing parent at level %d", i)
			}
		}
	})

	t.Run("Should handle children with mixed status and complex outputs", func(t *testing.T) {
		// Arrange - Parent with children in different states
		builder := shared.NewChildrenIndexBuilder()
		outputBuilder := shared.NewTaskOutputBuilder()

		parentExecID := core.MustNewID()
		child1ExecID := core.MustNewID()
		child2ExecID := core.MustNewID()
		child3ExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: parentExecID,
			Status:     core.StatusRunning,
		}

		// Children with different statuses and complex outputs
		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"parent-task": parentState,
				"child-success": {
					TaskID:     "child-success",
					TaskExecID: child1ExecID,
					Status:     core.StatusSuccess,
					Output: &core.Output{
						"result": "success data",
						"nested": map[string]any{
							"value": 42,
							"list":  []any{1, 2, 3},
						},
					},
				},
				"child-failed": {
					TaskID:     "child-failed",
					TaskExecID: child2ExecID,
					Status:     core.StatusFailed,
					Error:      &core.Error{Message: "test failure", Code: "TEST_FAILURE"},
					Output:     &core.Output{"partial": "data"},
				},
				"child-running": {
					TaskID:     "child-running",
					TaskExecID: child3ExecID,
					Status:     core.StatusRunning,
					// No output yet
				},
			},
		}

		childrenIndex := map[string][]string{
			string(parentExecID): {"child-success", "child-failed", "child-running"},
		}

		// Act - Build children context
		result := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			nil,
			outputBuilder,
			0,
		)

		// Assert - Should handle all children with different states
		require.NotNil(t, result)
		assert.Len(t, result, 3)

		// Check successful child
		successChild, ok := result["child-success"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "child-success", successChild["id"])
		assert.Equal(t, core.StatusSuccess, successChild["status"])
		assert.Contains(t, successChild, "output")

		// Check failed child
		failedChild, ok := result["child-failed"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "child-failed", failedChild["id"])
		assert.Equal(t, core.StatusFailed, failedChild["status"])
		assert.Contains(t, failedChild, "error")
		assert.Contains(t, failedChild, "output")

		// Check running child
		runningChild, ok := result["child-running"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "child-running", runningChild["id"])
		assert.Equal(t, core.StatusRunning, runningChild["status"])
		assert.Contains(t, runningChild, "output") // Should have empty output
	})

	t.Run("Should handle parent access with corrupted task configuration", func(t *testing.T) {
		// Arrange - Task with corrupted/missing config data
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		workflowState := &workflow.State{
			WorkflowID:     "corrupted-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"corrupted-task": {
					TaskID:     "corrupted-task",
					TaskExecID: core.MustNewID(),
					Status:     core.StatusRunning,
				},
			},
		}

		// Task config with minimal/corrupted data
		corruptedConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "corrupted-task",
				// Missing Type and other critical fields
			},
			// No other fields set
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"corrupted-task": corruptedConfig,
			},
			Variables: make(map[string]any),
		}

		// Act - Try to build parent context with corrupted config
		result := builder.BuildParentContext(ctx, corruptedConfig, 0)

		// Assert - Should handle gracefully without panicking
		if result != nil {
			assert.Equal(t, "corrupted-task", result[shared.IDKey])
			// Should have basic structure even with corrupted config
		}
	})

	t.Run("Should handle context building with nil workflow state", func(t *testing.T) {
		// Arrange - Nil workflow state
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "isolated-task",
				Type: task.TaskTypeBasic,
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: nil, // Nil workflow state
			TaskConfigs: map[string]*task.Config{
				"isolated-task": taskConfig,
			},
			Variables: make(map[string]any),
		}

		// Act - Try to build context with nil workflow state
		result := builder.BuildParentContext(ctx, taskConfig, 0)

		// Assert - Should handle gracefully
		// Result should be nil or contain basic task info
		if result != nil {
			assert.Equal(t, "isolated-task", result[shared.IDKey])
		}
	})

	t.Run("Should handle children building with empty children index", func(t *testing.T) {
		// Arrange - Parent with no children in index
		builder := shared.NewChildrenIndexBuilder()
		outputBuilder := shared.NewTaskOutputBuilder()

		parentState := &task.State{
			TaskID:     "lonely-parent",
			TaskExecID: core.MustNewID(),
			Status:     core.StatusRunning,
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"lonely-parent": parentState,
			},
		}

		// Empty children index
		emptyChildrenIndex := make(map[string][]string)

		// Act - Build children context with empty index
		result := builder.BuildChildrenContext(
			parentState,
			workflowState,
			emptyChildrenIndex,
			nil,
			outputBuilder,
			0,
		)

		// Assert - Should return empty result
		require.NotNil(t, result)
		assert.Empty(t, result)
	})
}
