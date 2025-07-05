package memory

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseHandler_NewResponseHandler(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatYAML)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)
	baseHandler := &shared.BaseResponseHandler{}

	t.Run("Should create handler with all valid dependencies", func(t *testing.T) {
		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)
		require.NotNil(t, handler)
		assert.Equal(t, templateEngine, handler.templateEngine)
		assert.Equal(t, contextBuilder, handler.contextBuilder)
		assert.Equal(t, baseHandler, handler.baseHandler)
	})

	t.Run("Should return error when baseHandler is nil", func(t *testing.T) {
		handler, err := NewResponseHandler(templateEngine, contextBuilder, nil)
		assert.Nil(t, handler)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create memory response handler: baseHandler is required but was nil")
	})

	t.Run("Should return error when templateEngine is nil", func(t *testing.T) {
		handler, err := NewResponseHandler(nil, contextBuilder, baseHandler)
		assert.Nil(t, handler)
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			"failed to create memory response handler: templateEngine is required but was nil")
	})

	t.Run("Should return error when contextBuilder is nil", func(t *testing.T) {
		handler, err := NewResponseHandler(templateEngine, nil, baseHandler)
		assert.Nil(t, handler)
		require.Error(t, err)
		assert.Contains(t, err.Error(),
			"failed to create memory response handler: contextBuilder is required but was nil")
	})
}

func TestResponseHandler_Type(t *testing.T) {
	t.Run("Should return TaskTypeMemory", func(t *testing.T) {
		templateEngine := tplengine.NewEngine(tplengine.FormatYAML)
		contextBuilder, err := shared.NewContextBuilder()
		require.NoError(t, err)
		baseHandler := &shared.BaseResponseHandler{}
		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)
		assert.Equal(t, task.TaskTypeMemory, handler.Type())
	})
}

func TestResponseHandler_HandleResponse(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatYAML)
	contextBuilder, err := shared.NewContextBuilder()
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("Should process valid memory task response", func(t *testing.T) {
		// Skip this test - it requires proper BaseResponseHandler setup
		t.Skip("Requires BaseResponseHandler with all dependencies")
	})

	t.Run("Should return validation error from base handler", func(t *testing.T) {
		// Test with nil input which should cause validation error
		baseHandler := &shared.BaseResponseHandler{}
		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)

		output, err := handler.HandleResponse(ctx, nil)
		assert.Nil(t, output)
		assert.Error(t, err)
	})

	t.Run("Should return error for incorrect task type", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeBasic, // Wrong type
				},
			},
			TaskState:      &task.State{},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		output, err := handler.HandleResponse(ctx, input)
		assert.Nil(t, output)
		require.Error(t, err)
		validationErr, ok := err.(*shared.ValidationError)
		require.True(t, ok)
		assert.Equal(t, "task_type", validationErr.Field)
		assert.Contains(t, validationErr.Message, "memory response handler received incorrect task type")
		assert.Contains(t, validationErr.Message, "expected 'memory', got 'basic'")
	})

	t.Run("Should handle processing error from base handler", func(t *testing.T) {
		// Skip this test - it requires proper BaseResponseHandler setup
		t.Skip("Requires BaseResponseHandler with all dependencies")
	})
}
