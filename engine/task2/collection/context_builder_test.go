package collection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestCollectionContextBuilder_NewContextBuilder(t *testing.T) {
	t.Run("Should create collection context builder", func(t *testing.T) {
		// Act
		builder := collection.NewContextBuilder()

		// Assert
		assert.NotNil(t, builder)
	})
}

func TestCollectionContextBuilder_TaskType(t *testing.T) {
	t.Run("Should return correct task type", func(t *testing.T) {
		// Arrange
		builder := collection.NewContextBuilder()

		// Act
		taskType := builder.TaskType()

		// Assert
		assert.Equal(t, task.TaskTypeCollection, taskType)
	})
}

func TestCollectionContextBuilder_BuildContext(t *testing.T) {
	// Setup
	builder := collection.NewContextBuilder()

	t.Run("Should build context for collection task", func(t *testing.T) {
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
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items: "test_items",
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
				Type: task.TaskTypeCollection,
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
				Type: task.TaskTypeCollection,
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

func TestCollectionContextBuilder_BuildIterationContext(t *testing.T) {
	// Setup
	builder := collection.NewContextBuilder()

	t.Run("Should build iteration context with item and index", func(t *testing.T) {
		// Arrange
		baseContext := &shared.NormalizationContext{
			Variables: map[string]any{
				"base_var": "base_value",
			},
			TaskConfigs:   make(map[string]*task.Config),
			ChildrenIndex: make(map[string][]string),
		}
		item := "test-item"
		index := 1

		// Act
		result, err := builder.BuildIterationContext(baseContext, item, index)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, baseContext.WorkflowState, result.WorkflowState)
		assert.Equal(t, baseContext.WorkflowConfig, result.WorkflowConfig)
		assert.Equal(t, baseContext.TaskConfig, result.TaskConfig)
		assert.Equal(t, baseContext.ParentTask, result.ParentTask)
		assert.Equal(t, baseContext.TaskConfigs, result.TaskConfigs)
		assert.Equal(t, baseContext.ChildrenIndex, result.ChildrenIndex)
		assert.Equal(t, baseContext.MergedEnv, result.MergedEnv)

		// Check variables
		require.NotNil(t, result.Variables)
		assert.Equal(t, "base_value", result.Variables["base_var"])
		assert.Equal(t, item, result.Variables[shared.ItemKey])
		assert.Equal(t, index, result.Variables[shared.IndexKey])

		// Check current input
		require.NotNil(t, result.CurrentInput)
		assert.Equal(t, item, (*result.CurrentInput)[shared.ItemKey])
		assert.Equal(t, index, (*result.CurrentInput)[shared.IndexKey])

		// Check input variable
		inputVar, exists := result.Variables["input"]
		assert.True(t, exists)
		assert.Equal(t, result.CurrentInput, inputVar)
	})

	t.Run("Should handle nil base variables", func(t *testing.T) {
		// Arrange
		baseContext := &shared.NormalizationContext{
			Variables: nil,
		}
		item := "test-item"
		index := 2

		// Act
		result, err := builder.BuildIterationContext(baseContext, item, index)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Variables)
		assert.Equal(t, item, result.Variables[shared.ItemKey])
		assert.Equal(t, index, result.Variables[shared.IndexKey])
	})

	t.Run("Should deep copy base variables to avoid shared references", func(t *testing.T) {
		// Arrange
		baseVars := map[string]any{
			"mutable_map": map[string]any{
				"nested_key": "original_value",
			},
		}
		baseContext := &shared.NormalizationContext{
			Variables: baseVars,
		}
		item := "test-item"
		index := 0

		// Act
		result, err := builder.BuildIterationContext(baseContext, item, index)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)

		// Modify the iteration context variables
		if mutableMap, ok := result.Variables["mutable_map"].(map[string]any); ok {
			mutableMap["nested_key"] = "modified_value"
		}

		// Original should remain unchanged
		originalMap := baseVars["mutable_map"].(map[string]any)
		assert.Equal(t, "original_value", originalMap["nested_key"])
	})

	t.Run("Should handle different item types", func(t *testing.T) {
		// Arrange
		baseContext := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		testCases := []struct {
			name  string
			item  any
			index int
		}{
			{"string item", "string-item", 0},
			{"int item", 42, 1},
			{"map item", map[string]any{"key": "value"}, 2},
			{"slice item", []string{"a", "b", "c"}, 3},
			{"nil item", nil, 4},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				result, err := builder.BuildIterationContext(baseContext, tc.item, tc.index)

				// Assert
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tc.item, result.Variables[shared.ItemKey])
				assert.Equal(t, tc.index, result.Variables[shared.IndexKey])
			})
		}
	})
}

func TestCollectionContextBuilder_BuildIterationContextWithProgress(t *testing.T) {
	// Setup
	builder := collection.NewContextBuilder()

	t.Run("Should build iteration context with progress state", func(t *testing.T) {
		// Arrange
		baseContext := &shared.NormalizationContext{
			Variables: map[string]any{
				"base_var": "base_value",
			},
		}
		item := "test-item"
		index := 1
		progressState := &task.ProgressState{
			TotalChildren: 10,
			SuccessCount:  3,
			FailedCount:   1,
			TerminalCount: 4,
			RunningCount:  2,
			PendingCount:  4,
		}

		// Act
		result, err := builder.BuildIterationContextWithProgress(baseContext, item, index, progressState)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, item, result.Variables[shared.ItemKey])
		assert.Equal(t, index, result.Variables[shared.IndexKey])

		// Check progress context
		progressCtx, exists := result.Variables["progress"]
		assert.True(t, exists)
		assert.NotNil(t, progressCtx)
	})

	t.Run("Should handle nil progress state", func(t *testing.T) {
		// Arrange
		baseContext := &shared.NormalizationContext{
			Variables: map[string]any{
				"base_var": "base_value",
			},
		}
		item := "test-item"
		index := 1

		// Act
		result, err := builder.BuildIterationContextWithProgress(baseContext, item, index, nil)

		// Assert
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, item, result.Variables[shared.ItemKey])
		assert.Equal(t, index, result.Variables[shared.IndexKey])

		// Should not have progress context
		_, exists := result.Variables["progress"]
		assert.False(t, exists)
	})

	t.Run("Should handle complex item structures", func(t *testing.T) {
		// Arrange - Test with complex but valid data structures
		baseContext := &shared.NormalizationContext{
			Variables: map[string]any{
				"complex_data": map[string]any{
					"nested": []string{"a", "b", "c"},
				},
			},
		}
		item := map[string]any{
			"id":   "complex-item",
			"data": []int{1, 2, 3},
		}
		index := 1
		progressState := &task.ProgressState{
			TotalChildren: 10,
		}

		// Act
		result, err := builder.BuildIterationContextWithProgress(baseContext, item, index, progressState)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, item, result.Variables[shared.ItemKey])
		assert.Equal(t, index, result.Variables[shared.IndexKey])
	})
}

func TestCollectionContextBuilder_EnrichContext(t *testing.T) {
	// Setup
	builder := collection.NewContextBuilder()

	t.Run("Should enrich context with base enrichment", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}
		taskState := &task.State{
			TaskID: string(core.MustNewID()),
			Status: core.StatusRunning,
		}

		// Act
		err := builder.EnrichContext(ctx, taskState)

		// Assert
		assert.NoError(t, err)
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
		// This might cause a panic in BaseContextBuilder, but that's expected behavior
		// The test ensures our method can handle the call
		assert.Error(t, err)
	})
}

func TestCollectionContextBuilder_ValidateContext(t *testing.T) {
	// Setup
	builder := collection.NewContextBuilder()

	t.Run("Should validate collection task with items field", func(t *testing.T) {
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
					Type: task.TaskTypeCollection,
				},
				CollectionConfig: task.CollectionConfig{
					Items: "test_items",
				},
			},
			Variables: make(map[string]any),
		}

		// Act
		err := builder.ValidateContext(ctx)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should return error for collection task missing items field", func(t *testing.T) {
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
					Type: task.TaskTypeCollection,
				},
				CollectionConfig: task.CollectionConfig{
					Items: "", // Missing items
				},
			},
			Variables: make(map[string]any),
		}

		// Act
		err := builder.ValidateContext(ctx)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "collection task config missing items field")
	})

	t.Run("Should skip validation for non-collection tasks", func(t *testing.T) {
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
					Type: task.TaskTypeBasic,
				},
			},
			Variables: make(map[string]any),
		}

		// Act
		err := builder.ValidateContext(ctx)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle nil task config", func(t *testing.T) {
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
			TaskConfig: nil,
			Variables:  make(map[string]any),
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
		// This might cause a panic in BaseContextBuilder, but that's expected behavior
		assert.Error(t, err)
	})
}
