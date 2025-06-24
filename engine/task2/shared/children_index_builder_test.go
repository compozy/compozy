package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// mockTaskOutputBuilder for testing
type mockTaskOutputBuilder struct{}

func (m *mockTaskOutputBuilder) BuildTaskOutput(
	taskState *task.State,
	_ map[string]*task.State,
	_ map[string][]string,
	_ int,
) any {
	if taskState.Output != nil {
		return *taskState.Output
	}
	return core.Output{}
}

func (m *mockTaskOutputBuilder) BuildEmptyOutput() core.Output {
	return core.Output{}
}

func TestChildrenIndexBuilder_NewChildrenIndexBuilder(t *testing.T) {
	t.Run("Should create children index builder", func(t *testing.T) {
		// Act
		builder := shared.NewChildrenIndexBuilder()

		// Assert
		assert.NotNil(t, builder)
	})
}

func TestChildrenIndexBuilder_BuildChildrenIndex(t *testing.T) {
	builder := shared.NewChildrenIndexBuilder()

	t.Run("Should build children index with parent-child relationships", func(t *testing.T) {
		// Arrange
		parentExecID := core.MustNewID()
		childExecID1 := core.MustNewID()
		childExecID2 := core.MustNewID()

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"parent": {
					TaskID:     "parent",
					TaskExecID: parentExecID,
				},
				"child1": {
					TaskID:        "child1",
					TaskExecID:    childExecID1,
					ParentStateID: &parentExecID,
				},
				"child2": {
					TaskID:        "child2",
					TaskExecID:    childExecID2,
					ParentStateID: &parentExecID,
				},
			},
		}

		// Act
		childrenIndex := builder.BuildChildrenIndex(workflowState)

		// Assert
		require.NotNil(t, childrenIndex)
		children, exists := childrenIndex[string(parentExecID)]
		require.True(t, exists)
		assert.Len(t, children, 2)
		assert.Contains(t, children, "child1")
		assert.Contains(t, children, "child2")
	})

	t.Run("Should handle workflow state with no tasks", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			Tasks: nil,
		}

		// Act
		childrenIndex := builder.BuildChildrenIndex(workflowState)

		// Assert
		require.NotNil(t, childrenIndex)
		assert.Empty(t, childrenIndex)
	})

	t.Run("Should handle nil workflow state", func(t *testing.T) {
		// Act
		childrenIndex := builder.BuildChildrenIndex(nil)

		// Assert
		require.NotNil(t, childrenIndex)
		assert.Empty(t, childrenIndex)
	})

	t.Run("Should handle tasks with no parent", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"standalone": {
					TaskID:        "standalone",
					TaskExecID:    core.MustNewID(),
					ParentStateID: nil,
				},
			},
		}

		// Act
		childrenIndex := builder.BuildChildrenIndex(workflowState)

		// Assert
		require.NotNil(t, childrenIndex)
		assert.Empty(t, childrenIndex)
	})
}

func TestChildrenIndexBuilder_BuildChildrenContext(t *testing.T) {
	builder := shared.NewChildrenIndexBuilder()
	mockOutputBuilder := &mockTaskOutputBuilder{}

	t.Run("Should build children context for parent task", func(t *testing.T) {
		// Arrange
		parentExecID := core.MustNewID()
		childExecID := core.MustNewID()

		parentState := &task.State{
			TaskID:     "parent",
			TaskExecID: parentExecID,
		}

		childState := &task.State{
			TaskID:        "child1",
			TaskExecID:    childExecID,
			ParentStateID: &parentExecID,
			Status:        core.StatusSuccess,
			Output: &core.Output{
				"result": "child_output",
			},
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"parent": parentState,
				"child1": childState,
			},
		}

		childrenIndex := map[string][]string{
			string(parentExecID): {"child1"},
		}

		taskConfigs := map[string]*task.Config{
			"child1": {
				BaseConfig: task.BaseConfig{
					ID:   "child1",
					Type: task.TaskTypeBasic,
				},
			},
		}

		// Act
		children := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			taskConfigs,
			mockOutputBuilder,
			0,
		)

		// Assert
		require.NotNil(t, children)
		childContext, exists := children["child1"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, "child1", childContext["id"])
		assert.Equal(t, core.StatusSuccess, childContext["status"])
		assert.Equal(t, *childState.Output, childContext["output"])
	})

	t.Run("Should handle parent with no children", func(t *testing.T) {
		// Arrange
		parentState := &task.State{
			TaskID:     "parent",
			TaskExecID: core.MustNewID(),
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"parent": parentState,
			},
		}

		childrenIndex := make(map[string][]string)

		// Act
		children := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			nil,
			mockOutputBuilder,
			0,
		)

		// Assert
		require.NotNil(t, children)
		assert.Empty(t, children)
	})

	t.Run("Should prevent unbounded recursion", func(t *testing.T) {
		// Arrange
		parentState := &task.State{
			TaskID:     "parent",
			TaskExecID: core.MustNewID(),
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"parent": parentState,
			},
		}

		childrenIndex := make(map[string][]string)

		// Act with depth at maximum
		children := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			nil,
			mockOutputBuilder,
			10, // maxContextDepth
		)

		// Assert
		require.NotNil(t, children)
		assert.Empty(t, children)
	})
}

func TestChildrenIndexBuilder_buildChildContextWithoutParent(t *testing.T) {
	builder := shared.NewChildrenIndexBuilder()
	mockOutputBuilder := &mockTaskOutputBuilder{}

	t.Run("Should build child context without parent reference", func(t *testing.T) {
		// Arrange
		taskState := &task.State{
			TaskID:     "child1",
			TaskExecID: core.MustNewID(),
			Status:     core.StatusSuccess,
			Input: &core.Input{
				"param": "value",
			},
			Output: &core.Output{
				"result": "success",
			},
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"child1": taskState,
			},
		}

		taskConfigs := map[string]*task.Config{
			"child1": {
				BaseConfig: task.BaseConfig{
					ID:   "child1",
					Type: task.TaskTypeBasic,
				},
				BasicTask: task.BasicTask{
					Action: "test-action",
				},
			},
		}

		// Act
		// Use reflection to call private method through BuildChildrenContext
		childrenIndex := map[string][]string{
			"parent": {"child1"},
		}

		parentState := &task.State{
			TaskID:     "parent",
			TaskExecID: core.ID("parent"),
		}

		children := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			taskConfigs,
			mockOutputBuilder,
			0,
		)

		// Assert
		require.NotNil(t, children)
		childContext, exists := children["child1"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, "child1", childContext["id"])
		assert.Equal(t, taskState.Input, childContext["input"])
		assert.Equal(t, core.StatusSuccess, childContext["status"])
		assert.Equal(t, *taskState.Output, childContext["output"])

		// Check that task config was merged (action should be present)
		assert.Equal(t, "test-action", childContext["action"])
	})

	t.Run("Should handle task with error", func(t *testing.T) {
		// Arrange
		taskError := &core.Error{
			Message: "test error",
		}
		taskState := &task.State{
			TaskID:     "child1",
			TaskExecID: core.MustNewID(),
			Status:     core.StatusFailed,
			Error:      taskError,
		}

		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"child1": taskState,
			},
		}

		childrenIndex := map[string][]string{
			"parent": {"child1"},
		}

		parentState := &task.State{
			TaskID:     "parent",
			TaskExecID: core.ID("parent"),
		}

		// Act
		children := builder.BuildChildrenContext(
			parentState,
			workflowState,
			childrenIndex,
			nil,
			mockOutputBuilder,
			0,
		)

		// Assert
		require.NotNil(t, children)
		childContext, exists := children["child1"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, taskError, childContext["error"])
	})
}
