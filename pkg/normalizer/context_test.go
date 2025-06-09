package normalizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func TestContextBuilder_BuildCollectionContext(t *testing.T) {
	contextBuilder := NewContextBuilder()

	t.Run("Should build context with workflow input and output", func(t *testing.T) {
		workflowInput := core.Input{
			"param1": "value1",
			"param2": 42,
		}
		workflowOutput := core.Output{
			"result": "success",
			"count":  3,
		}

		workflowState := &workflow.State{
			Input:  &workflowInput,
			Output: &workflowOutput,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "test-task",
			},
		}

		result := contextBuilder.BuildCollectionContext(workflowState, taskConfig)

		require.NotNil(t, result)
		assert.Equal(t, workflowInput, result["input"])
		assert.Equal(t, workflowOutput, result["output"])
	})

	t.Run("Should build context with task 'with' parameters", func(t *testing.T) {
		workflowState := &workflow.State{}

		withParams := core.Input{
			"customParam": "customValue",
			"iterations":  5,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				With: &withParams,
			},
		}

		result := contextBuilder.BuildCollectionContext(workflowState, taskConfig)

		require.NotNil(t, result)
		assert.Equal(t, "customValue", result["customParam"])
		assert.Equal(t, 5, result["iterations"])
	})

	t.Run("Should handle nil workflow input/output", func(t *testing.T) {
		workflowState := &workflow.State{
			Input:  nil,
			Output: nil,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "test-task",
			},
		}

		result := contextBuilder.BuildCollectionContext(workflowState, taskConfig)

		require.NotNil(t, result)
		assert.NotContains(t, result, "input")
		assert.NotContains(t, result, "output")
	})

	t.Run("Should merge workflow state and task config", func(t *testing.T) {
		workflowInput := core.Input{
			"globalParam": "globalValue",
		}
		workflowOutput := core.Output{
			"globalResult": "done",
		}

		workflowState := &workflow.State{
			Input:  &workflowInput,
			Output: &workflowOutput,
		}

		withParams := core.Input{
			"taskParam": "taskValue",
			"override":  "fromTask",
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				With: &withParams,
			},
		}

		result := contextBuilder.BuildCollectionContext(workflowState, taskConfig)

		require.NotNil(t, result)

		// Should have workflow input/output
		assert.Equal(t, workflowInput, result["input"])
		assert.Equal(t, workflowOutput, result["output"])

		// Should have task-specific parameters
		assert.Equal(t, "taskValue", result["taskParam"])
		assert.Equal(t, "fromTask", result["override"])
	})

	t.Run("Should handle empty task config", func(t *testing.T) {
		workflowInput := core.Input{
			"param": "value",
		}

		workflowState := &workflow.State{
			Input: &workflowInput,
		}

		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "test-task",
				With: nil,
			},
		}

		result := contextBuilder.BuildCollectionContext(workflowState, taskConfig)

		require.NotNil(t, result)
		assert.Equal(t, workflowInput, result["input"])
		assert.NotContains(t, result, "output")
	})
}
