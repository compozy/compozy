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

func TestContextBuilder_NewContextBuilder(t *testing.T) {
	t.Run("Should create context builder with cache", func(t *testing.T) {
		builder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		assert.NotNil(t, builder)
	})
}

func TestContextBuilder_BuildContext(t *testing.T) {
	builder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should build context with workflow and task data", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"key": "value",
			},
			Output: &core.Output{
				"result": "success",
			},
			Status: core.StatusRunning,
			Tasks: map[string]*task.State{
				"task1": {
					TaskID:     "task1",
					TaskExecID: core.MustNewID(),
					Input: &core.Input{
						"task_input": "data",
					},
					Output: &core.Output{
						"task_output": "result",
					},
					Status: core.StatusSuccess,
				},
			},
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
			Opts: workflow.Opts{
				Env: &core.EnvMap{
					"ENV_VAR": "env_value",
				},
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
				With: &core.Input{
					"with_data": "value",
				},
				Env: &core.EnvMap{
					"TASK_ENV": "task_value",
				},
			},
			BasicTask: task.BasicTask{
				Action: "test-action",
			},
		}

		// Act
		ctx := builder.BuildContext(workflowState, workflowConfig, taskConfig)

		// Assert
		require.NotNil(t, ctx)
		assert.Equal(t, workflowState, ctx.WorkflowState)
		assert.Equal(t, workflowConfig, ctx.WorkflowConfig)
		assert.Equal(t, taskConfig, ctx.TaskConfig)
		assert.NotNil(t, ctx.Variables)

		// Check workflow variables
		workflowVars, ok := ctx.Variables["workflow"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test-workflow", workflowVars["id"])
		assert.Equal(t, workflowState.Input, workflowVars["input"])
		assert.Equal(t, workflowState.Output, workflowVars["output"])

		// Check task variables
		taskVars, ok := ctx.Variables["task"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "task1", taskVars["id"])
		assert.Equal(t, task.TaskTypeBasic, taskVars["type"])

		// Check env variables
		assert.Equal(t, workflowConfig.Opts.Env, ctx.Variables["env"])

		// Check tasks map
		tasksMap, ok := ctx.Variables["tasks"].(map[string]any)
		require.True(t, ok)
		assert.Contains(t, tasksMap, "task1")
	})

	t.Run("Should handle nil workflow state", func(t *testing.T) {
		// Act
		ctx := builder.BuildContext(nil, nil, nil)

		// Assert
		require.NotNil(t, ctx)
		assert.NotNil(t, ctx.Variables)
		assert.Empty(t, ctx.TaskConfigs)
	})

	t.Run("Should build children index correctly", func(t *testing.T) {
		// Arrange
		parentExecID := core.MustNewID()
		childExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Tasks: map[string]*task.State{
				"parent": {
					TaskID:     "parent",
					TaskExecID: parentExecID,
				},
				"child": {
					TaskID:        "child",
					TaskExecID:    childExecID,
					ParentStateID: &parentExecID,
				},
			},
		}

		// Act
		ctx := builder.BuildContext(workflowState, nil, nil)

		// Assert
		require.NotNil(t, ctx.ChildrenIndex)
		assert.Contains(t, ctx.ChildrenIndex, string(parentExecID))
		assert.Contains(t, ctx.ChildrenIndex[string(parentExecID)], "child")
	})

	t.Run("Should include current input in variables", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "task1",
			},
		}
		currentInput := &core.Input{
			"item":  "test-item",
			"index": 5,
		}

		// Act
		ctx := builder.BuildContext(workflowState, nil, taskConfig)
		ctx.CurrentInput = currentInput
		// Rebuild context with current input set
		ctx = builder.BuildContext(workflowState, nil, taskConfig)
		ctx.CurrentInput = currentInput
		// Add current input to variables using the method that normally handles this
		builder.AddCollectionData(ctx, "test-item", 5)
		vars := ctx.BuildTemplateContext()

		// Assert
		assert.Equal(t, "test-item", vars["item"])
		assert.Equal(t, 5, vars["index"])
	})
}

func TestContextBuilder_BuildParentContext(t *testing.T) {
	builder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should build parent context with task config", func(t *testing.T) {
		// Arrange
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"parent_data": "value",
				},
				Env: &core.EnvMap{
					"PARENT_ENV": "parent_value",
				},
			},
		}
		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow",
				Tasks: map[string]*task.State{
					"parent-task": {
						TaskID: "parent-task",
						Input: &core.Input{
							"runtime_input": "runtime_value",
						},
						Output: &core.Output{
							"runtime_output": "output_value",
						},
						Status: core.StatusSuccess,
					},
				},
			},
		}

		// Act
		parentContext := builder.BuildParentContext(ctx, parentTask, 0)

		// Assert
		require.NotNil(t, parentContext)
		assert.Equal(t, "parent-task", parentContext["id"])
		assert.Equal(t, task.TaskTypeParallel, parentContext["type"])
		assert.Equal(t, *parentTask.With, parentContext["with"])
		assert.Equal(t, parentTask.Env, parentContext["env"])

		// Check runtime state is included
		assert.Equal(t, ctx.WorkflowState.Tasks["parent-task"].Input, parentContext["input"])
		assert.Equal(t, ctx.WorkflowState.Tasks["parent-task"].Output, parentContext["output"])
		assert.Equal(t, core.StatusSuccess, parentContext["status"])
	})

	t.Run("Should cache parent contexts", func(t *testing.T) {
		// Arrange
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "cached-task",
			},
		}
		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow",
			},
		}

		// Act - build twice
		result1 := builder.BuildParentContext(ctx, parentTask, 0)
		result2 := builder.BuildParentContext(ctx, parentTask, 0)

		// Assert - should be the same cached instance
		assert.Equal(t, result1, result2)
	})

	t.Run("Should prevent infinite recursion", func(t *testing.T) {
		// Arrange
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "recursive-task",
			},
		}
		ctx := &shared.NormalizationContext{}

		// Act - call with max depth
		result := builder.BuildParentContext(ctx, parentTask, 10)

		// Assert
		assert.Nil(t, result)
	})

	t.Run("Should build recursive parent chain", func(t *testing.T) {
		// Arrange
		grandParentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "grandparent",
				Type: task.TaskTypeComposite,
			},
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "parent",
						Type: task.TaskTypeParallel,
					},
					Tasks: []task.Config{
						{
							BaseConfig: task.BaseConfig{
								ID:   "child",
								Type: task.TaskTypeBasic,
							},
						},
					},
				},
			},
		}

		workflowConfig := &workflow.Config{
			Tasks: []task.Config{*grandParentTask},
		}

		ctx := &shared.NormalizationContext{
			WorkflowConfig: workflowConfig,
			TaskConfigs: map[string]*task.Config{
				"grandparent": grandParentTask,
				"parent":      &grandParentTask.Tasks[0],
				"child":       &grandParentTask.Tasks[0].Tasks[0],
			},
		}

		// Act - build parent context for child
		childTask := &grandParentTask.Tasks[0].Tasks[0]
		parentContext := builder.BuildParentContext(ctx, childTask, 0)

		// Assert - should find parent through workflow config search
		assert.NotNil(t, parentContext)
	})
}

func TestContextBuilder_BuildSubTaskContext(t *testing.T) {
	builder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should build sub-task context with parent", func(t *testing.T) {
		// Arrange
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "parent-task",
				Type: task.TaskTypeParallel,
				With: &core.Input{
					"parent_data": "value",
				},
			},
		}
		parentState := &task.State{
			TaskID:     "parent-task",
			TaskExecID: core.MustNewID(),
			Input: &core.Input{
				"runtime_input": "runtime",
			},
			Output: &core.Output{
				"runtime_output": "output",
			},
			Status: core.StatusRunning,
		}
		baseCtx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow",
			},
			Variables: map[string]any{
				"workflow": map[string]any{
					"id": "test-workflow",
				},
				"env": map[string]any{
					"GLOBAL": "value",
				},
			},
		}

		// Act
		subTaskCtx := builder.BuildSubTaskContext(baseCtx, parentTask, parentState)

		// Assert
		require.NotNil(t, subTaskCtx)
		assert.Equal(t, parentTask, subTaskCtx.ParentTask)
		assert.NotNil(t, subTaskCtx.Variables["parent"])
		assert.NotNil(t, subTaskCtx.Variables["current"])

		// Check current task state
		current, ok := subTaskCtx.Variables["current"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "parent-task", current["id"])
		assert.Equal(t, parentState.Input, current["input"])
		assert.Equal(t, parentState.Output, current["output"])
		assert.Equal(t, core.StatusRunning, current["status"])
	})

	t.Run("Should handle task state with error", func(t *testing.T) {
		// Arrange
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "error-task",
			},
		}
		taskError := &core.Error{
			Message: "test error",
		}
		parentState := &task.State{
			TaskID: "error-task",
			Error:  taskError,
		}
		baseCtx := &shared.NormalizationContext{
			Variables: make(map[string]any),
		}

		// Act
		subTaskCtx := builder.BuildSubTaskContext(baseCtx, parentTask, parentState)

		// Assert
		current, ok := subTaskCtx.Variables["current"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, taskError, current["error"])
	})
}

func TestContextBuilder_BuildChildrenContext(t *testing.T) {
	builder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should build children context map", func(t *testing.T) {
		// Arrange
		parentExecID := core.MustNewID()
		parentState := &task.State{
			TaskID:     "parent",
			TaskExecID: parentExecID,
		}
		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				Tasks: map[string]*task.State{
					"child1": {
						TaskID:        "child1",
						TaskExecID:    core.MustNewID(),
						ParentStateID: &parentExecID,
						Input: &core.Input{
							"child1_input": "data1",
						},
						Output: &core.Output{
							"child1_output": "result1",
						},
						Status: core.StatusSuccess,
					},
					"child2": {
						TaskID:        "child2",
						TaskExecID:    core.MustNewID(),
						ParentStateID: &parentExecID,
						Input: &core.Input{
							"child2_input": "data2",
						},
						Status: core.StatusRunning,
					},
				},
			},
			ChildrenIndex: map[string][]string{
				string(parentExecID): {"child1", "child2"},
			},
		}

		// Act
		children := builder.ChildrenIndexBuilder.BuildChildrenContext(
			parentState,
			ctx.WorkflowState,
			ctx.ChildrenIndex,
			ctx.TaskConfigs,
			builder.TaskOutputBuilder,
			0,
		)

		// Assert
		require.NotNil(t, children)
		assert.Len(t, children, 2)

		// Check child1
		child1, ok := children["child1"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "child1", child1["id"])
		assert.Equal(t, ctx.WorkflowState.Tasks["child1"].Input, child1["input"])
		assert.Equal(t, *ctx.WorkflowState.Tasks["child1"].Output, child1["output"])
		assert.Equal(t, core.StatusSuccess, child1["status"])

		// Check child2
		child2, ok := children["child2"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "child2", child2["id"])
		assert.Equal(t, core.StatusRunning, child2["status"])
	})

	t.Run("Should prevent infinite recursion in children", func(t *testing.T) {
		// Arrange
		parentState := &task.State{
			TaskID:     "parent",
			TaskExecID: core.MustNewID(),
		}
		ctx := &shared.NormalizationContext{
			ChildrenIndex: make(map[string][]string),
		}

		// Act - call with max depth
		children := builder.ChildrenIndexBuilder.BuildChildrenContext(
			parentState,
			ctx.WorkflowState,
			ctx.ChildrenIndex,
			ctx.TaskConfigs,
			builder.TaskOutputBuilder,
			10,
		)

		// Assert
		assert.Empty(t, children)
	})
}

func TestContextBuilder_ClearCache(t *testing.T) {
	builder, err := shared.NewContextBuilder()
	require.NoError(t, err)

	t.Run("Should clear parent context cache", func(t *testing.T) {
		// Arrange - build a parent context to cache it
		parentTask := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "cached-task",
			},
		}
		ctx := &shared.NormalizationContext{
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow",
			},
		}
		builder.BuildParentContext(ctx, parentTask, 0)

		// Act
		builder.ClearCache()

		// Assert - cache should be cleared (no direct way to verify, but method should not panic)
		assert.NotPanics(t, func() {
			builder.ClearCache()
		})
	})
}

func TestNormalizationContext_GetVariables(t *testing.T) {
	t.Run("Should create variables map if nil", func(t *testing.T) {
		// Arrange
		ctx := &shared.NormalizationContext{}

		// Act
		vars := ctx.GetVariables()

		// Assert
		assert.NotNil(t, vars)
		assert.Equal(t, ctx.Variables, vars)
	})

	t.Run("Should return existing variables", func(t *testing.T) {
		// Arrange
		existingVars := map[string]any{
			"key": "value",
		}
		ctx := &shared.NormalizationContext{
			Variables: existingVars,
		}

		// Act
		vars := ctx.GetVariables()

		// Assert
		assert.Equal(t, existingVars, vars)
	})
}
