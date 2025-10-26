package parallel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/parallel"
	"github.com/compozy/compozy/engine/workflow"
)

func TestParallelContextBuilder_NewContextBuilder(t *testing.T) {
	t.Run("Should create parallel context builder", func(t *testing.T) {
		// Act
		builder := parallel.NewContextBuilder(t.Context())

		// Assert
		assert.NotNil(t, builder)
	})
}

func TestParallelContextBuilder_TaskType(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		builder := parallel.NewContextBuilder(t.Context())

		// Act
		taskType := builder.TaskType()

		// Assert
		assert.Equal(t, task.TaskTypeParallel, taskType)
	})
}

func TestParallelContextBuilder_BuildContext(t *testing.T) {
	// Setup
	builder := parallel.NewContextBuilder(t.Context())

	t.Run("Should build context for parallel task", func(t *testing.T) {
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
				Type: task.TaskTypeParallel,
			},
			ParallelTask: task.ParallelTask{
				Strategy:   task.StrategyWaitAll,
				MaxWorkers: 5,
			},
		}

		// Act
		context := builder.BuildContext(t.Context(), workflowState, workflowConfig, taskConfig)

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
				Type: task.TaskTypeParallel,
			},
		}

		// Act
		context := builder.BuildContext(t.Context(), nil, workflowConfig, taskConfig)

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
				Type: task.TaskTypeParallel,
			},
		}

		// Act
		context := builder.BuildContext(t.Context(), workflowState, nil, taskConfig)

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
		context := builder.BuildContext(t.Context(), workflowState, workflowConfig, nil)

		// Assert
		require.NotNil(t, context)
		assert.Equal(t, workflowState, context.WorkflowState)
		assert.Equal(t, workflowConfig, context.WorkflowConfig)
		assert.Nil(t, context.TaskConfig)
	})

	t.Run("Should handle all nil parameters", func(t *testing.T) {
		// Act
		context := builder.BuildContext(t.Context(), nil, nil, nil)

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

func TestParallelContextBuilder_EnrichContext(t *testing.T) {
	// Setup
	builder := parallel.NewContextBuilder(t.Context())

	t.Run("Should enrich context with task state", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeParallel,
			},
		}
		context := builder.BuildContext(t.Context(), nil, nil, taskConfig)
		taskState := &task.State{
			Status: core.StatusRunning,
		}

		// Act
		err := builder.EnrichContext(context, taskState)

		// Assert
		assert.NoError(t, err)
		// Verify task status was added
		taskMap := context.Variables["task"].(map[string]any)
		assert.Equal(t, core.StatusRunning, taskMap["status"])
	})

	t.Run("Should handle nil context", func(t *testing.T) {
		// Arrange
		taskState := &task.State{
			Status: core.StatusRunning,
		}

		// Act
		err := builder.EnrichContext(nil, taskState)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("Should handle nil task state", func(t *testing.T) {
		// Arrange
		context := builder.BuildContext(t.Context(), nil, nil, nil)

		// Act
		err := builder.EnrichContext(context, nil)

		// Assert
		assert.NoError(t, err)
	})
}

func TestParallelContextBuilder_ValidateContext(t *testing.T) {
	// Setup
	builder := parallel.NewContextBuilder(t.Context())

	t.Run("Should validate complete context", func(t *testing.T) {
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
				Type: task.TaskTypeParallel,
			},
		}
		context := builder.BuildContext(t.Context(), workflowState, workflowConfig, taskConfig)

		// Act
		err := builder.ValidateContext(context)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle nil context", func(t *testing.T) {
		// Act
		err := builder.ValidateContext(nil)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("Should validate minimal context", func(t *testing.T) {
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
		context := builder.BuildContext(t.Context(), workflowState, workflowConfig, nil)

		// Act
		err := builder.ValidateContext(context)

		// Assert
		assert.NoError(t, err)
	})
}
