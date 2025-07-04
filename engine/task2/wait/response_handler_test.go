package wait

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestWaitResponseHandler_NewResponseHandler(t *testing.T) {
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

func TestWaitResponseHandler_Type(t *testing.T) {
	t.Run("Should return wait task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeWait, handler.Type())
	})
}

func TestWaitResponseHandler_HandleResponse_Validation(t *testing.T) {
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

		result, err := handler.HandleResponse(context.TODO(), input)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "handler type does not match task type")

		var validationErr *shared.ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_type", validationErr.Field)
	})
}

func TestWaitResponseHandler_ValidateWaitCompletion(t *testing.T) {
	t.Run("Should validate successful completion with output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		state := &task.State{
			TaskExecID: "test-exec-id",
			TaskID:     "test-task-id",
			Status:     core.StatusSuccess,
			Output: &core.Output{
				"signal": "received",
			},
		}

		err := handler.ValidateWaitCompletion(state)

		assert.NoError(t, err)
	})

	t.Run("Should validate successful completion without output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		state := &task.State{
			TaskExecID: "test-exec-id",
			TaskID:     "test-task-id",
			Status:     core.StatusSuccess,
			Output:     nil, // No output is OK
		}

		err := handler.ValidateWaitCompletion(state)

		assert.NoError(t, err)
	})

	t.Run("Should validate failed completion", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		state := &task.State{
			TaskExecID: "test-exec-id",
			TaskID:     "test-task-id",
			Status:     core.StatusFailed,
			Error:      &core.Error{Message: "wait timeout"},
		}

		err := handler.ValidateWaitCompletion(state)

		assert.NoError(t, err) // Current implementation doesn't return errors
	})

	t.Run("Should validate running completion", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		state := &task.State{
			TaskExecID: "test-exec-id",
			TaskID:     "test-task-id",
			Status:     core.StatusRunning,
		}

		err := handler.ValidateWaitCompletion(state)

		assert.NoError(t, err) // Current implementation doesn't return errors
	})

	t.Run("Should handle nil state gracefully", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		// Test that nil state returns an error instead of panicking
		err := handler.ValidateWaitCompletion(nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "state cannot be nil")
	})
}
