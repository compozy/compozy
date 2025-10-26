package shared_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
)

func TestBuildParentContext_EdgeCases(t *testing.T) {
	t.Run("Should handle task with no parent (root task)", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		rootTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "root-task",
				Type: task.TaskTypeBasic,
				With: &core.Input{"rootParam": "rootValue"},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"root-task": {
					TaskID:        "root-task",
					TaskExecID:    core.MustNewID(),
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionBasic,
					Input:         &core.Input{"rootParam": "rootValue"},
					ParentStateID: nil, // No parent
				},
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"root-task": rootTask,
			},
			Variables: make(map[string]any),
		}

		// Act
		result := builder.BuildParentContext(t.Context(), ctx, rootTask, 0)

		// Assert
		require.NotNil(t, result)
		assert.Equal(t, "root-task", result[shared.IDKey])
		assert.Equal(t, task.TaskTypeBasic, result[shared.TypeKey])
		assert.Equal(t, core.StatusRunning, result[shared.StatusKey])

		// Should not have parent key
		_, hasParent := result[shared.ParentKey]
		assert.False(t, hasParent, "Root task should not have parent")
	})

	t.Run("Should handle missing parent task state gracefully", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		parentID := core.MustNewID()
		childTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeBasic,
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"child-task": {
					TaskID:        "child-task",
					TaskExecID:    core.MustNewID(),
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parentID, // Parent doesn't exist in state
				},
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"child-task": childTask,
			},
			Variables: make(map[string]any),
		}

		// Act
		result := builder.BuildParentContext(t.Context(), ctx, childTask, 0)

		// Assert
		require.NotNil(t, result)
		assert.Equal(t, "child-task", result[shared.IDKey])

		// Should have parent with error
		parentCtx, hasParent := result[shared.ParentKey].(map[string]any)
		assert.True(t, hasParent, "Should have parent context even if parent state missing")
		assert.Contains(t, parentCtx, "error")
		assert.Equal(t, "parent task state not found", parentCtx["error"])
	})

	t.Run("Should handle missing parent task config gracefully", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		parentID := core.MustNewID()
		childTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeBasic,
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"parent-task": {
					TaskID:        "parent-task",
					TaskExecID:    parentID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionComposite,
				},
				"child-task": {
					TaskID:        "child-task",
					TaskExecID:    core.MustNewID(),
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &parentID,
				},
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"child-task": childTask,
				// parent-task config is missing
			},
			Variables: make(map[string]any),
		}

		// Act
		result := builder.BuildParentContext(t.Context(), ctx, childTask, 0)

		// Assert
		require.NotNil(t, result)

		// Should still have parent context with state data
		parentCtx, hasParent := result[shared.ParentKey].(map[string]any)
		assert.True(t, hasParent)
		assert.Equal(t, "parent-task", parentCtx[shared.IDKey])
		assert.Equal(t, core.StatusRunning, parentCtx[shared.StatusKey])
	})

	t.Run("Should handle maximum depth correctly", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		// Create a deep hierarchy (20 levels)
		taskConfigs := make(map[string]*task.Config)
		taskStates := make(map[string]*task.State)

		var previousExecID *core.ID
		for i := range 20 {
			taskID := string(rune('A'+i)) + "-task"
			execID := core.MustNewID()

			taskConfigs[taskID] = &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   taskID,
					Type: task.TaskTypeBasic,
					With: &core.Input{"level": i},
				},
			}

			taskStates[taskID] = &task.State{
				TaskID:        taskID,
				TaskExecID:    execID,
				Status:        core.StatusRunning,
				ExecutionType: task.ExecutionBasic,
				ParentStateID: previousExecID,
				Input:         &core.Input{"level": i},
			}

			previousExecID = &execID
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks:          taskStates,
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs:   taskConfigs,
			Variables:     make(map[string]any),
		}

		// Act - Get context for the deepest task
		deepestTask := taskConfigs["T-task"]
		result := builder.BuildParentContext(t.Context(), ctx, deepestTask, 0)

		// Assert
		require.NotNil(t, result)

		// Count the depth of parent chain
		depth := 0
		current := result
		for current != nil && depth < 25 { // Safety limit
			depth++
			if parent, ok := current[shared.ParentKey].(map[string]any); ok {
				current = parent
			} else {
				break
			}
		}

		// Should respect max depth limit (10 by default)
		limits := shared.GetGlobalConfigLimits(t.Context())
		assert.LessOrEqual(t, depth, limits.MaxParentDepth+1, "Should not exceed max parent depth")
	})

	t.Run("Should merge runtime state with config data correctly", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "merge-test",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"configParam": "configValue",
					"override":    "fromConfig",
				},
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"merge-test": {
					TaskID:        "merge-test",
					TaskExecID:    core.MustNewID(),
					Status:        core.StatusSuccess,
					ExecutionType: task.ExecutionBasic,
					Input: &core.Input{
						"runtimeParam": "runtimeValue",
						"override":     "fromRuntime",
					},
					Output: &core.Output{
						"result": "success",
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"merge-test": taskConfig,
			},
			Variables: make(map[string]any),
		}

		// Act
		result := builder.BuildParentContext(t.Context(), ctx, taskConfig, 0)

		// Assert
		require.NotNil(t, result)

		// BuildParentContext sets the runtime Input separately from With
		// Check WithKey for config data (stored as dereferenced map)
		withMap, ok := result[shared.WithKey].(core.Input)
		assert.True(t, ok)
		if ok {
			assert.Equal(t, "configValue", withMap["configParam"], "Should have config param in With")
			assert.Equal(t, "fromConfig", withMap["override"], "Should have config value in With")
		}

		// Check InputKey for runtime data
		inputMap, ok := result[shared.InputKey].(*core.Input)
		assert.True(t, ok)
		if ok {
			assert.Equal(t, "runtimeValue", (*inputMap)["runtimeParam"], "Should have runtime param in Input")
			assert.Equal(t, "fromRuntime", (*inputMap)["override"], "Should have runtime value in Input")
		}

		// Should have runtime status and output
		assert.Equal(t, core.StatusSuccess, result[shared.StatusKey])
		assert.NotNil(t, result[shared.OutputKey])
	})

	t.Run("Should handle collection task with item context", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		collectionExecID := core.MustNewID()
		childExecID := core.MustNewID()

		collectionTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "collection-task",
				Type: task.TaskTypeCollection,
			},
			CollectionConfig: task.CollectionConfig{
				Items:    `["item1", "item2"]`,
				ItemVar:  "current",
				IndexVar: "idx",
			},
		}

		childTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "child-task",
				Type: task.TaskTypeBasic,
			},
		}

		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Tasks: map[string]*task.State{
				"collection-task": {
					TaskID:        "collection-task",
					TaskExecID:    collectionExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionCollection,
					Input:         &core.Input{"items": []any{"item1", "item2"}},
				},
				"child-task": {
					TaskID:        "child-task",
					TaskExecID:    childExecID,
					Status:        core.StatusRunning,
					ExecutionType: task.ExecutionBasic,
					ParentStateID: &collectionExecID,
					Input: &core.Input{
						"current": "item1",
						"idx":     0,
						"item":    "item1",
						"index":   0,
					},
				},
			},
		}

		ctx := &shared.NormalizationContext{
			WorkflowState: workflowState,
			TaskConfigs: map[string]*task.Config{
				"collection-task": collectionTask,
				"child-task":      childTask,
			},
			Variables: make(map[string]any),
		}

		// Act - Get context for child of collection
		result := builder.BuildParentContext(t.Context(), ctx, childTask, 0)

		// Assert
		require.NotNil(t, result)

		// Should have collection item context
		inputPtr, ok := result[shared.InputKey].(*core.Input)
		assert.True(t, ok)
		if ok {
			assert.Equal(t, "item1", (*inputPtr)["current"])
			assert.Equal(t, 0, (*inputPtr)["idx"])
		}

		// Parent should be collection
		parentCtx, hasParent := result[shared.ParentKey].(map[string]any)
		assert.True(t, hasParent)
		assert.Equal(t, "collection-task", parentCtx[shared.IDKey])
		assert.Equal(t, task.TaskTypeCollection, parentCtx[shared.TypeKey])
	})

	t.Run("Should handle nil normalization context", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				Type: task.TaskTypeBasic,
			},
		}

		// Act & Assert
		assert.Panics(t, func() {
			_ = builder.BuildParentContext(t.Context(), nil, taskConfig, 0)
		})
	})

	t.Run("Should handle nil task config", func(t *testing.T) {
		// Arrange
		builder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)

		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{},
			TaskConfigs:   make(map[string]*task.Config),
			Variables:     make(map[string]any),
		}

		// Act
		result := builder.BuildParentContext(t.Context(), ctx, nil, 0)

		// Assert
		assert.Nil(t, result)
	})
}
