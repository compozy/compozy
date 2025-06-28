package wait_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/wait"
	"github.com/compozy/compozy/engine/workflow"
)

func TestWaitContextBuilder_NewContextBuilder(t *testing.T) {
	t.Run("Should create wait context builder", func(t *testing.T) {
		// Act
		builder := wait.NewContextBuilder()

		// Assert
		assert.NotNil(t, builder)
	})
}

func TestWaitContextBuilder_TaskType(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		builder := wait.NewContextBuilder()

		// Act
		taskType := builder.TaskType()

		// Assert
		assert.Equal(t, task.TaskTypeWait, taskType)
	})
}

func TestWaitContextBuilder_BuildContext(t *testing.T) {
	// Setup
	builder := wait.NewContextBuilder()

	t.Run("Should build context for wait task", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
			Tasks:          make(map[string]*task.State),
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "test-signal",
			},
		}

		// Act
		context := builder.BuildContext(workflowState, workflowConfig, taskConfig)

		// Assert
		require.NotNil(t, context)
		assert.Equal(t, workflowState, context.WorkflowState)
		assert.Equal(t, workflowConfig, context.WorkflowConfig)
		assert.Equal(t, taskConfig, context.TaskConfig)
		assert.NotNil(t, context.TaskConfigs)
		assert.NotNil(t, context.Variables)
		assert.NotNil(t, context.ChildrenIndex)
	})

	t.Run("Should handle nil workflow state", func(t *testing.T) {
		// Arrange
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeWait,
			},
		}

		// Act
		context := builder.BuildContext(nil, workflowConfig, taskConfig)

		// Assert
		require.NotNil(t, context)
		assert.Nil(t, context.WorkflowState)
		assert.Equal(t, workflowConfig, context.WorkflowConfig)
		assert.Equal(t, taskConfig, context.TaskConfig)
	})

	t.Run("Should handle nil workflow config", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
			Tasks:          make(map[string]*task.State),
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeWait,
			},
		}

		// Act
		context := builder.BuildContext(workflowState, nil, taskConfig)

		// Assert
		require.NotNil(t, context)
		assert.Equal(t, workflowState, context.WorkflowState)
		assert.Nil(t, context.WorkflowConfig)
		assert.Equal(t, taskConfig, context.TaskConfig)
	})

	t.Run("Should handle nil task config", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
			Tasks:          make(map[string]*task.State),
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}

		// Act
		context := builder.BuildContext(workflowState, workflowConfig, nil)

		// Assert
		require.NotNil(t, context)
		assert.Equal(t, workflowState, context.WorkflowState)
		assert.Equal(t, workflowConfig, context.WorkflowConfig)
		assert.Nil(t, context.TaskConfig)
	})

	t.Run("Should handle all nil parameters", func(t *testing.T) {
		// Act
		context := builder.BuildContext(nil, nil, nil)

		// Assert
		require.NotNil(t, context)
		assert.Nil(t, context.WorkflowState)
		assert.Nil(t, context.WorkflowConfig)
		assert.Nil(t, context.TaskConfig)
		assert.NotNil(t, context.TaskConfigs)
		assert.NotNil(t, context.Variables)
		assert.NotNil(t, context.ChildrenIndex)
	})
}
