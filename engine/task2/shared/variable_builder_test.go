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

func TestVariableBuilder_NewVariableBuilder(t *testing.T) {
	t.Run("Should create variable builder", func(t *testing.T) {
		// Act
		builder := shared.NewVariableBuilder()

		// Assert
		assert.NotNil(t, builder)
	})
}

func TestVariableBuilder_BuildBaseVariables(t *testing.T) {
	builder := shared.NewVariableBuilder()

	t.Run("Should build base variables with workflow and task data", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
			Input: &core.Input{
				"global": "value",
			},
			Output: &core.Output{
				"result": "success",
			},
			Status: core.StatusRunning,
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
					"param": "value",
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
		vars := builder.BuildBaseVariables(workflowState, workflowConfig, taskConfig)

		// Assert
		require.NotNil(t, vars)

		// Check workflow data
		workflowData, exists := vars["workflow"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, "test-workflow", workflowData["id"])
		assert.Equal(t, workflowState.Input, workflowData["input"])
		assert.Equal(t, workflowState.Output, workflowData["output"])
		assert.Equal(t, core.StatusRunning, workflowData["status"])
		assert.Equal(t, workflowConfig, workflowData["config"])

		// Check task data
		taskData, exists := vars["task"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, "task1", taskData["id"])
		assert.Equal(t, task.TaskTypeBasic, taskData["type"])
		assert.Equal(t, "test-action", taskData["action"])
		assert.Equal(t, taskConfig.With, taskData["with"])
		assert.Equal(t, taskConfig.Env, taskData["env"])

		// Check env data
		envData, exists := vars["env"]
		require.True(t, exists)
		assert.Equal(t, workflowConfig.Opts.Env, envData)
	})

	t.Run("Should handle nil workflow state", func(t *testing.T) {
		// Arrange
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "task1",
				Type: task.TaskTypeBasic,
			},
		}

		// Act
		vars := builder.BuildBaseVariables(nil, nil, taskConfig)

		// Assert
		require.NotNil(t, vars)

		// Check task data exists
		taskData, exists := vars["task"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, "task1", taskData["id"])

		// Check workflow data doesn't exist
		_, exists = vars["workflow"]
		assert.False(t, exists)
	})

	t.Run("Should handle nil task config", func(t *testing.T) {
		// Arrange
		workflowState := &workflow.State{
			WorkflowID: "test-workflow",
		}

		// Act
		vars := builder.BuildBaseVariables(workflowState, nil, nil)

		// Assert
		require.NotNil(t, vars)

		// Check workflow data exists
		workflowData, exists := vars["workflow"].(map[string]any)
		require.True(t, exists)
		assert.Equal(t, "test-workflow", workflowData["id"])

		// Check task data doesn't exist
		_, exists = vars["task"]
		assert.False(t, exists)
	})
}

func TestVariableBuilder_AddTasksToVariables(t *testing.T) {
	builder := shared.NewVariableBuilder()

	t.Run("Should add tasks to variables", func(t *testing.T) {
		// Arrange
		vars := make(map[string]any)
		workflowState := &workflow.State{
			Tasks: map[string]*task.State{
				"task1": {
					TaskID: "task1",
					Status: core.StatusSuccess,
				},
			},
		}
		tasksMap := map[string]any{
			"task1": map[string]any{
				"id":     "task1",
				"status": core.StatusSuccess,
			},
		}

		// Act
		builder.AddTasksToVariables(vars, workflowState, tasksMap)

		// Assert
		tasks, exists := vars["tasks"]
		require.True(t, exists)
		assert.Equal(t, tasksMap, tasks)
	})

	t.Run("Should handle nil workflow state", func(t *testing.T) {
		// Arrange
		vars := make(map[string]any)
		tasksMap := map[string]any{
			"task1": map[string]any{
				"id": "task1",
			},
		}

		// Act
		builder.AddTasksToVariables(vars, nil, tasksMap)

		// Assert
		_, exists := vars["tasks"]
		assert.False(t, exists)
	})
}

func TestVariableBuilder_AddCurrentInputToVariables(t *testing.T) {
	builder := shared.NewVariableBuilder()

	t.Run("Should add current input to variables", func(t *testing.T) {
		// Arrange
		vars := make(map[string]any)
		currentInput := &core.Input{
			"data":  "value",
			"item":  "collection_item",
			"index": 5,
		}

		// Act
		builder.AddCurrentInputToVariables(vars, currentInput)

		// Assert
		input, exists := vars["input"]
		require.True(t, exists)
		assert.Equal(t, currentInput, input)

		item, exists := vars["item"]
		require.True(t, exists)
		assert.Equal(t, "collection_item", item)

		index, exists := vars["index"]
		require.True(t, exists)
		assert.Equal(t, 5, index)
	})

	t.Run("Should handle nil current input", func(t *testing.T) {
		// Arrange
		vars := make(map[string]any)

		// Act
		builder.AddCurrentInputToVariables(vars, nil)

		// Assert
		_, exists := vars["input"]
		assert.False(t, exists)
		_, exists = vars["item"]
		assert.False(t, exists)
		_, exists = vars["index"]
		assert.False(t, exists)
	})
}

func TestVariableBuilder_CopyVariables(t *testing.T) {
	builder := shared.NewVariableBuilder()

	t.Run("Should copy variables map", func(t *testing.T) {
		// Arrange
		source := map[string]any{
			"key1": "value1",
			"key2": map[string]any{
				"nested": "value",
			},
		}

		// Act
		copied, err := builder.CopyVariables(source)
		require.NoError(t, err)

		// Assert
		require.NotNil(t, copied)
		assert.Equal(t, source, copied)
		// Ensure it's a different map instance
		// Cannot use NotSame on maps created with make(), check modification isolation instead

		// Modify copied and ensure original is unchanged
		copied["key3"] = "new_value"
		_, exists := source["key3"]
		assert.False(t, exists)
	})

	t.Run("Should handle nil source", func(t *testing.T) {
		// Act
		copied, err := builder.CopyVariables(nil)
		require.NoError(t, err)

		// Assert
		require.NotNil(t, copied)
		assert.Empty(t, copied)
	})
}

func TestVariableBuilder_AddParentToVariables(t *testing.T) {
	builder := shared.NewVariableBuilder()

	t.Run("Should add parent context to variables", func(t *testing.T) {
		// Arrange
		vars := make(map[string]any)
		parentContext := map[string]any{
			"id":   "parent_task",
			"type": "parallel",
		}

		// Act
		builder.AddParentToVariables(vars, parentContext)

		// Assert
		parent, exists := vars["parent"]
		require.True(t, exists)
		assert.Equal(t, parentContext, parent)
	})

	t.Run("Should handle nil parent context", func(t *testing.T) {
		// Arrange
		vars := make(map[string]any)

		// Act
		builder.AddParentToVariables(vars, nil)

		// Assert
		_, exists := vars["parent"]
		assert.False(t, exists)
	})
}
