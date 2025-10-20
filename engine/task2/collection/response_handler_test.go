package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestCollectionResponseHandler_NewResponseHandler(t *testing.T) {
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

func TestCollectionResponseHandler_Type(t *testing.T) {
	t.Run("Should return collection task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeCollection, handler.Type())
	})
}

func TestCollectionResponseHandler_HandleResponse_Validation(t *testing.T) {
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

func TestCollectionResponseHandler_applyCollectionContext(t *testing.T) {
	t.Run("Should handle nil task input", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{},
			},
		}
		// Set With to nil
		input.TaskConfig.With = nil

		// Should not panic
		assert.NotPanics(t, func() {
			handler.applyCollectionContext(t.Context(), input)
		})
	})

	t.Run("Should handle empty task input", func(t *testing.T) {
		contextBuilder := &shared.ContextBuilder{}
		handler := NewResponseHandler(nil, contextBuilder, nil)

		taskInput := &core.Input{}
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{},
			},
		}
		// Set With properly
		input.TaskConfig.With = taskInput

		// Should not panic
		assert.NotPanics(t, func() {
			handler.applyCollectionContext(t.Context(), input)
		})
	})
}

func TestCollectionResponseHandler_HandleSubtaskResponse(t *testing.T) {
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
}

func TestCollectionResponseHandler_ValidateCollectionOutput(t *testing.T) {
	t.Run("Should handle nil output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		err := handler.ValidateCollectionOutput(nil)

		assert.NoError(t, err)
	})

	t.Run("Should validate non-nil output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		output := &core.Output{
			"items": []any{"item1", "item2"},
			"count": 2,
		}

		err := handler.ValidateCollectionOutput(output)

		assert.NoError(t, err)
	})
}
