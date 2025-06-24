package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
)

func TestTaskOutputBuilder_NewTaskOutputBuilder(t *testing.T) {
	t.Run("Should create task output builder", func(t *testing.T) {
		// Act
		builder := shared.NewTaskOutputBuilder()

		// Assert
		assert.NotNil(t, builder)
	})
}

func TestTaskOutputBuilder_BuildTaskOutput(t *testing.T) {
	builder := shared.NewTaskOutputBuilder()

	t.Run("Should build output for simple task", func(t *testing.T) {
		// Arrange
		taskState := &task.State{
			TaskID:     "task1",
			TaskExecID: core.MustNewID(),
			Output: &core.Output{
				"result": "success",
				"data":   123,
			},
		}

		// Act
		output := builder.BuildTaskOutput(taskState, nil, nil, 0)

		// Assert
		expectedOutput := *taskState.Output
		assert.Equal(t, expectedOutput, output)
	})

	t.Run("Should build nested output for parent task with children", func(t *testing.T) {
		// Arrange
		parentExecID := core.MustNewID()
		childExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:        "parent",
			TaskExecID:    parentExecID,
			ExecutionType: task.ExecutionParallel, // This makes CanHaveChildren() return true
			Output: &core.Output{
				"parent_result": "parent_success",
			},
		}

		childState := &task.State{
			TaskID:        "child1",
			TaskExecID:    childExecID,
			ParentStateID: &parentExecID,
			Status:        core.StatusSuccess,
			Output: &core.Output{
				"child_result": "child_success",
			},
		}

		workflowTasks := map[string]*task.State{
			"parent": parentState,
			"child1": childState,
		}

		childrenIndex := map[string][]string{
			string(parentExecID): {"child1"},
		}

		// Act
		output := builder.BuildTaskOutput(parentState, workflowTasks, childrenIndex, 0)

		// Assert
		nestedOutput, ok := output.(map[string]any)
		require.True(t, ok)

		// Check parent's own output
		parentOutput, exists := nestedOutput["output"]
		require.True(t, exists)
		assert.Equal(t, *parentState.Output, parentOutput)

		// Check child's output
		childOutput, exists := nestedOutput["child1"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, *childState.Output, childOutput["output"])
		assert.Equal(t, core.StatusSuccess, childOutput["status"])
	})

	t.Run("Should handle task with no output", func(t *testing.T) {
		// Arrange
		taskState := &task.State{
			TaskID:     "task1",
			TaskExecID: core.MustNewID(),
			Output:     nil,
		}

		// Act
		output := builder.BuildTaskOutput(taskState, nil, nil, 0)

		// Assert
		assert.Equal(t, core.Output{}, output)
	})

	t.Run("Should handle nil task state", func(t *testing.T) {
		// Act
		output := builder.BuildTaskOutput(nil, nil, nil, 0)

		// Assert
		assert.Nil(t, output)
	})

	t.Run("Should prevent unbounded recursion", func(t *testing.T) {
		// Arrange
		parentState := &task.State{
			TaskID:        "parent",
			TaskExecID:    core.MustNewID(),
			ExecutionType: task.ExecutionParallel, // Has children
		}

		// Act with depth at maximum
		output := builder.BuildTaskOutput(parentState, nil, nil, 10) // maxContextDepth

		// Assert
		assert.Nil(t, output)
	})

	t.Run("Should include child error in nested output", func(t *testing.T) {
		// Arrange
		parentExecID := core.MustNewID()
		childExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:        "parent",
			TaskExecID:    parentExecID,
			ExecutionType: task.ExecutionParallel, // Has children
		}

		childError := &core.Error{
			Message: "child failed",
		}

		childState := &task.State{
			TaskID:        "child1",
			TaskExecID:    childExecID,
			ParentStateID: &parentExecID,
			Status:        core.StatusFailed,
			Error:         childError,
		}

		workflowTasks := map[string]*task.State{
			"parent": parentState,
			"child1": childState,
		}

		childrenIndex := map[string][]string{
			string(parentExecID): {"child1"},
		}

		// Act
		output := builder.BuildTaskOutput(parentState, workflowTasks, childrenIndex, 0)

		// Assert
		nestedOutput, ok := output.(map[string]any)
		require.True(t, ok)

		childOutput, exists := nestedOutput["child1"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, childError, childOutput["error"])
		assert.Equal(t, core.StatusFailed, childOutput["status"])
	})

	t.Run("Should handle parent with children but no children index", func(t *testing.T) {
		// Arrange
		parentState := &task.State{
			TaskID:        "parent",
			TaskExecID:    core.MustNewID(),
			ExecutionType: task.ExecutionParallel, // Has children
			Output: &core.Output{
				"parent_result": "success",
			},
		}

		// Act - no children index provided
		output := builder.BuildTaskOutput(parentState, nil, nil, 0)

		// Assert
		nestedOutput, ok := output.(map[string]any)
		require.True(t, ok)

		// Should only have parent's own output
		parentOutput, exists := nestedOutput["output"]
		require.True(t, exists)
		assert.Equal(t, *parentState.Output, parentOutput)

		// Should not have any child outputs
		assert.Len(t, nestedOutput, 1) // Only "output" key
	})
}

func TestTaskOutputBuilder_BuildEmptyOutput(t *testing.T) {
	builder := shared.NewTaskOutputBuilder()

	t.Run("Should build empty output", func(t *testing.T) {
		// Act
		output := builder.BuildEmptyOutput()

		// Assert
		assert.NotNil(t, output)
		assert.Equal(t, core.Output{}, output)
	})
}
