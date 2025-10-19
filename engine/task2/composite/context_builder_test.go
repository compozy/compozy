package composite_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/composite"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestCompositeContextBuilder_NewContextBuilder(t *testing.T) {
	t.Run("Should create composite context builder", func(t *testing.T) {
		// Act
		builder := composite.NewContextBuilder(t.Context())

		// Assert
		assert.NotNil(t, builder)
	})
}

func TestCompositeContextBuilder_TaskType(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		builder := composite.NewContextBuilder(t.Context())

		// Act
		taskType := builder.TaskType()

		// Assert
		assert.Equal(t, task.TaskTypeComposite, taskType)
	})
}

func TestCompositeContextBuilder_BuildContext(t *testing.T) {
	// Setup
	builder := composite.NewContextBuilder(t.Context())

	t.Run("Should build context for composite task", func(t *testing.T) {
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
				Type: task.TaskTypeComposite,
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
				Type: task.TaskTypeComposite,
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
				Type: task.TaskTypeComposite,
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

func TestCompositeContextBuilder_EnrichContext(t *testing.T) {
	// Setup
	builder := composite.NewContextBuilder(t.Context())

	t.Run("Should enrich context with base enrichment", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"task": map[string]any{}, // Initialize task map
			},
		}
		taskState := &task.State{
			TaskID: string(core.MustNewID()),
			Status: core.StatusRunning,
		}

		// Act
		err := builder.EnrichContext(ctx, taskState)

		// Assert
		assert.NoError(t, err)
		// Verify task status was added
		taskMap := ctx.Variables["task"].(map[string]any)
		assert.Equal(t, core.StatusRunning, taskMap["status"])
	})

	t.Run("Should handle nil task state", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		err := builder.EnrichContext(ctx, nil)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle nil context", func(t *testing.T) {
		// Arrange
		taskState := &task.State{
			TaskID: string(core.MustNewID()),
			Status: core.StatusRunning,
		}

		// Act
		err := builder.EnrichContext(nil, taskState)

		// Assert
		assert.Error(t, err)
	})
}

func TestCompositeContextBuilder_ValidateContext(t *testing.T) {
	// Setup
	builder := composite.NewContextBuilder(t.Context())

	t.Run("Should validate composite task context", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: core.MustNewID(),
				Status:         core.StatusRunning,
				Tasks:          make(map[string]*task.State),
			},
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeComposite,
				},
			},
			Variables: make(map[string]any),
		}

		// Act
		err := builder.ValidateContext(ctx)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle nil context", func(t *testing.T) {
		// Act
		err := builder.ValidateContext(nil)

		// Assert
		assert.Error(t, err)
	})
}
