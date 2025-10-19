package composite

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestCompositeResponseHandler_NewResponseHandler(t *testing.T) {
	t.Run("Should create handler with dependencies", func(t *testing.T) {
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}

		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)

		assert.NotNil(t, handler)
		assert.Equal(t, templateEngine, handler.templateEngine)
		assert.Equal(t, contextBuilder, handler.contextBuilder)
		assert.Equal(t, baseHandler, handler.baseHandler)
	})
}

func TestCompositeResponseHandler_Type(t *testing.T) {
	t.Run("Should return composite task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeComposite, handler.Type())
	})
}

func TestCompositeResponseHandler_HandleResponse_Validation(t *testing.T) {
	t.Run("Should return error for wrong task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		// Provide full valid input except for wrong type
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeBasic, // Wrong type
				},
			},
			TaskState:      &task.State{},      // Valid state
			WorkflowConfig: &workflow.Config{}, // Valid workflow config
			WorkflowState:  &workflow.State{},  // Valid workflow state
		}

		result, err := handler.HandleResponse(t.Context(), input)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "handler type does not match task type")

		var validationErr *shared.ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_type", validationErr.Field)
	})
}

func TestCompositeResponseHandler_HandleSubtaskResponse(t *testing.T) {
	t.Run("Should create subtask response with child state", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		parentState := &task.State{
			TaskExecID: "parent-exec-id",
			Status:     core.StatusRunning,
		}

		childState := &task.State{
			TaskExecID: "child-exec-id",
			Status:     core.StatusSuccess,
			Output:     &core.Output{"result": "child-output"},
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "child-task-id",
			},
		}

		result, err := handler.HandleSubtaskResponse(
			t.Context(),
			parentState,
			childState,
			childConfig,
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "child-task-id", result.TaskID)
		assert.Equal(t, childState.Output, result.Output)
		assert.Equal(t, childState.Error, result.Error)
		assert.Equal(t, childState.Status, result.Status)
		assert.Equal(t, childState, result.State)
	})

	t.Run("Should handle failed child task", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		childState := &task.State{
			TaskExecID: "child-exec-id",
			Status:     core.StatusFailed,
			Error:      &core.Error{Message: "child task failed"},
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "failed-child-task",
			},
		}

		result, err := handler.HandleSubtaskResponse(
			t.Context(),
			nil,
			childState,
			childConfig,
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "failed-child-task", result.TaskID)
		assert.Equal(t, core.StatusFailed, result.Status)
		assert.Equal(t, childState.Error, result.Error)
		assert.Equal(t, childState, result.State)
	})

	t.Run("Should handle nil output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		childState := &task.State{
			TaskExecID: "child-exec-id",
			Status:     core.StatusSuccess,
			Output:     nil,
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "child-task-no-output",
			},
		}

		result, err := handler.HandleSubtaskResponse(
			t.Context(),
			nil,
			childState,
			childConfig,
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "child-task-no-output", result.TaskID)
		assert.Nil(t, result.Output)
		assert.Equal(t, core.StatusSuccess, result.Status)
	})
}
