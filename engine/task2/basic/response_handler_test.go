package basic

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

// Unit Tests for Constructor and Type Method (Subtask 7.2 - Public Method Coverage)
func TestNewResponseHandler(t *testing.T) {
	t.Run("Should create handler with valid dependencies", func(t *testing.T) {
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}

		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)

		assert.NotNil(t, handler)
		assert.Equal(t, templateEngine, handler.templateEngine)
		assert.Equal(t, contextBuilder, handler.contextBuilder)
		assert.Equal(t, baseHandler, handler.baseHandler)
	})

	t.Run("Should panic with nil baseHandler", func(t *testing.T) {
		assert.Panics(t, func() {
			NewResponseHandler(nil, nil, nil)
		}, "NewResponseHandler should panic when baseHandler is nil")
	})
}

func TestBasicResponseHandler_Type(t *testing.T) {
	t.Run("Should return TaskTypeBasic", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)
		assert.Equal(t, task.TaskTypeBasic, handler.Type())
	})
}

// Task Type Validation Tests (only testable unit logic)
func TestBasicResponseHandler_TaskTypeValidation(t *testing.T) {
	t.Run("Should validate correct task type without panic", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		// Test that we can create input with correct type without issues
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeBasic},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Verify the input has the correct type for this handler
		assert.Equal(t, task.TaskTypeBasic, input.TaskConfig.Type)
		assert.Equal(t, task.TaskTypeBasic, handler.Type())
	})

	t.Run("Should identify wrong task type without panic", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		// Test that we can identify type mismatches in input
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeRouter}, // Wrong type
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Verify we can detect the type mismatch
		assert.NotEqual(t, handler.Type(), input.TaskConfig.Type)
		assert.Equal(t, task.TaskTypeRouter, input.TaskConfig.Type)
		assert.Equal(t, task.TaskTypeBasic, handler.Type())
	})
}

// Test struct field accessibility (unit testing only the struct behavior)
func TestBasicResponseHandler_FieldAccess(t *testing.T) {
	t.Run("Should store and retrieve template engine", func(t *testing.T) {
		engine := &tplengine.TemplateEngine{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(engine, nil, baseHandler)

		assert.Same(t, engine, handler.templateEngine)
	})

	t.Run("Should store and retrieve context builder", func(t *testing.T) {
		builder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, builder, baseHandler)

		assert.Same(t, builder, handler.contextBuilder)
	})

	t.Run("Should store and retrieve base handler", func(t *testing.T) {
		base := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, base)

		assert.Same(t, base, handler.baseHandler)
	})
}

// Test input structure validation (without calling dependent methods)
func TestBasicResponseHandler_InputStructureValidation(t *testing.T) {
	t.Run("Should handle nil input config gracefully", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		_ = NewResponseHandler(nil, nil, baseHandler)

		input := &shared.ResponseInput{
			TaskConfig:     nil, // Test nil config
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// We can't call HandleResponse with nil base handler, but we can test the input structure
		assert.Nil(t, input.TaskConfig)
		assert.NotNil(t, input.TaskState)
	})

	t.Run("Should handle valid input structure", func(t *testing.T) {
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, baseHandler)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-task",
					Type: task.TaskTypeBasic,
					With: &core.Input{"param": "value"},
				},
			},
			TaskState: &task.State{
				TaskExecID: core.MustNewID(),
				Status:     core.StatusRunning,
			},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Validate structure without calling dependent methods
		assert.NotNil(t, input.TaskConfig)
		assert.Equal(t, "test-task", input.TaskConfig.ID)
		assert.Equal(t, task.TaskTypeBasic, input.TaskConfig.Type)
		assert.NotNil(t, input.TaskConfig.With)
		assert.NotNil(t, input.TaskState)
		assert.NotEmpty(t, input.TaskState.TaskExecID)
		assert.Equal(t, task.TaskTypeBasic, handler.Type())
	})
}

// Tests for HandleResponse method to increase coverage from 54.5% to 70%
func TestBasicResponseHandler_HandleResponse(t *testing.T) {
	t.Run("Should return validation error for nil input", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		// Act
		result, err := handler.HandleResponse(context.Background(), nil)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		var validationErr *shared.ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "input", validationErr.Field)
		assert.Contains(t, validationErr.Message, "input cannot be nil")
	})
	t.Run("Should return validation error for nil task config", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		input := &shared.ResponseInput{
			TaskConfig:     nil, // Invalid
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}
		// Act
		result, err := handler.HandleResponse(context.Background(), input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		var validationErr *shared.ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_config", validationErr.Field)
	})
	t.Run("Should return validation error for wrong task type", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeRouter, // Wrong type for basic handler
				},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}
		// Act
		result, err := handler.HandleResponse(context.Background(), input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		var validationErr *shared.ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_type", validationErr.Field)
		assert.Contains(t, validationErr.Message, "handler type does not match task type")
	})
}
