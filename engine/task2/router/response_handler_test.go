package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestRouterResponseHandler_NewResponseHandler(t *testing.T) {
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

func TestRouterResponseHandler_Type(t *testing.T) {
	t.Run("Should return router task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeRouter, handler.Type())
	})
}

func TestRouterResponseHandler_HandleResponse_Validation(t *testing.T) {
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

func TestRouterResponseHandler_ValidateRoutingDecision(t *testing.T) {
	t.Run("Should validate nil output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		err := handler.ValidateRoutingDecision(nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "router output cannot be nil")
	})

	t.Run("Should validate non-nil output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		output := &core.Output{
			"route":    "next-task",
			"decision": "go-right",
		}

		err := handler.ValidateRoutingDecision(output)

		assert.NoError(t, err)
	})
}

func TestRouterResponseHandler_setNextTaskFromRoute(t *testing.T) {
	t.Run("Should handle nil output", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{}
		response := &shared.ResponseOutput{
			State: &task.State{Output: nil},
		}

		err := handler.setNextTaskFromRoute(input, response)

		assert.NoError(t, err)
		assert.Nil(t, input.NextTaskOverride)
	})

	t.Run("Should handle missing route_taken field", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{}
		response := &shared.ResponseOutput{
			State: &task.State{
				Output: &core.Output{
					"other_field": "value",
				},
			},
		}

		err := handler.setNextTaskFromRoute(input, response)

		assert.NoError(t, err)
		assert.Nil(t, input.NextTaskOverride)
	})

	t.Run("Should handle invalid route_taken type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{}
		response := &shared.ResponseOutput{
			State: &task.State{
				Output: &core.Output{
					shared.FieldRouteTaken: 123, // Invalid type
				},
			},
		}

		err := handler.setNextTaskFromRoute(input, response)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "route_taken must be a string")
	})

	t.Run("Should handle route not found in workflow", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{
			WorkflowConfig: &workflow.Config{
				Tasks: []task.Config{
					{BaseConfig: task.BaseConfig{ID: "task1"}},
					{BaseConfig: task.BaseConfig{ID: "task2"}},
				},
			},
		}
		response := &shared.ResponseOutput{
			State: &task.State{
				Output: &core.Output{
					shared.FieldRouteTaken: "nonexistent-task",
				},
			},
		}

		err := handler.setNextTaskFromRoute(input, response)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "route 'nonexistent-task' not found in workflow tasks")
	})

	t.Run("Should set next task override when route found", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		targetTask := task.Config{BaseConfig: task.BaseConfig{ID: "target-task"}}
		input := &shared.ResponseInput{
			WorkflowConfig: &workflow.Config{
				Tasks: []task.Config{
					{BaseConfig: task.BaseConfig{ID: "task1"}},
					targetTask,
					{BaseConfig: task.BaseConfig{ID: "task3"}},
				},
			},
		}
		response := &shared.ResponseOutput{
			State: &task.State{
				Output: &core.Output{
					shared.FieldRouteTaken: "target-task",
				},
			},
		}

		err := handler.setNextTaskFromRoute(input, response)

		require.NoError(t, err)
		require.NotNil(t, input.NextTaskOverride)
		assert.Equal(t, "target-task", input.NextTaskOverride.ID)
	})

	t.Run("Should handle empty workflow tasks", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{
			WorkflowConfig: &workflow.Config{
				Tasks: []task.Config{},
			},
		}
		response := &shared.ResponseOutput{
			State: &task.State{
				Output: &core.Output{
					shared.FieldRouteTaken: "any-task",
				},
			},
		}

		err := handler.setNextTaskFromRoute(input, response)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "route 'any-task' not found in workflow tasks")
	})
}
