package aggregate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestAggregateResponseHandler_NewResponseHandler(t *testing.T) {
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

func TestAggregateResponseHandler_Type(t *testing.T) {
	t.Run("Should return aggregate task type", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)
		assert.Equal(t, task.TaskTypeAggregate, handler.Type())
	})
}

func TestAggregateResponseHandler_HandleResponse_Validation(t *testing.T) {
	t.Run("Should return error for wrong task type", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

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

func TestAggregateResponseHandler_validateAggregationResult(t *testing.T) {
	t.Run("Should validate nil output", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		err := handler.validateAggregationResult(nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "aggregate output cannot be nil")
	})

	t.Run("Should validate empty output", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		output := &core.Output{}

		err := handler.validateAggregationResult(output)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "aggregate output is empty")
	})

	t.Run("Should validate output with aggregated field", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		output := &core.Output{
			shared.FieldAggregated: []any{"item1", "item2"},
		}

		err := handler.validateAggregationResult(output)

		assert.NoError(t, err)
	})

	t.Run("Should validate output with other data", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		output := &core.Output{
			"total":  42,
			"count":  2,
			"result": "success",
		}

		err := handler.validateAggregationResult(output)

		assert.NoError(t, err)
	})
}

func TestAggregateResponseHandler_HandleAggregateCompletion(t *testing.T) {
	t.Run("Should set output when nil", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		state := &task.State{
			TaskExecID: "test-exec-id",
			TaskID:     "test-task-id",
			Output:     nil,
		}

		aggregatedData := map[string]any{
			"total": 100,
			"items": []string{"a", "b", "c"},
		}

		err := handler.HandleAggregateCompletion(t.Context(), state, aggregatedData)

		assert.NoError(t, err)
		assert.NotNil(t, state.Output)
		assert.Equal(t, 100, (*state.Output)["total"])
		assert.Equal(t, []string{"a", "b", "c"}, (*state.Output)["items"])
	})

	t.Run("Should merge with existing output", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		existingOutput := core.Output{
			"existing": "value",
			"count":    5,
		}

		state := &task.State{
			TaskExecID: "test-exec-id",
			TaskID:     "test-task-id",
			Output:     &existingOutput,
		}

		aggregatedData := map[string]any{
			"total": 100,
			"count": 10, // This should overwrite existing
		}

		err := handler.HandleAggregateCompletion(t.Context(), state, aggregatedData)

		assert.NoError(t, err)
		assert.NotNil(t, state.Output)
		assert.Equal(t, "value", (*state.Output)["existing"])
		assert.Equal(t, 100, (*state.Output)["total"])
		assert.Equal(t, 10, (*state.Output)["count"]) // Overwritten
	})

	t.Run("Should handle empty aggregated data", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		state := &task.State{
			TaskExecID: "test-exec-id",
			TaskID:     "test-task-id",
			Output:     nil,
		}

		aggregatedData := map[string]any{}

		err := handler.HandleAggregateCompletion(t.Context(), state, aggregatedData)

		assert.NoError(t, err)
		assert.NotNil(t, state.Output)
		assert.Empty(t, *state.Output)
	})
}
