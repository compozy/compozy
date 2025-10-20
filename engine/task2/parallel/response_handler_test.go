package parallel

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestParallelResponseHandler_NewResponseHandler(t *testing.T) {
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

	t.Run("Should handle nil dependencies gracefully", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)
		assert.NotNil(t, handler)
		assert.Nil(t, handler.templateEngine)
		assert.Nil(t, handler.contextBuilder)
		assert.Nil(t, handler.baseHandler)
	})
}

func TestParallelResponseHandler_Type(t *testing.T) {
	t.Run("Should return parallel task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeParallel, handler.Type())
	})
}

// Task Type Validation Tests (only testable unit logic)
func TestParallelResponseHandler_TaskTypeValidation(t *testing.T) {
	t.Run("Should validate correct task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeParallel},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Verify the input has the correct type for this handler
		assert.Equal(t, task.TaskTypeParallel, input.TaskConfig.Type)
		assert.Equal(t, task.TaskTypeParallel, handler.Type())
	})

	t.Run("Should identify wrong task type", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{Type: task.TaskTypeBasic}, // Wrong type
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Verify we can detect the type mismatch
		assert.NotEqual(t, handler.Type(), input.TaskConfig.Type)
		assert.Equal(t, task.TaskTypeBasic, input.TaskConfig.Type)
		assert.Equal(t, task.TaskTypeParallel, handler.Type())
	})
}

func TestParallelResponseHandler_HandleSubtaskResponse(t *testing.T) {
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
			task.StrategyWaitAll,
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "child-task-id", result.TaskID)
		assert.Equal(t, childState.Output, result.Output)
		assert.Equal(t, childState.Error, result.Error)
		assert.Equal(t, childState.Status, result.Status)
		assert.Equal(t, childState, result.State)
	})

	t.Run("Should handle child task with error", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		childError := &core.Error{Message: "child task failed"}
		childState := &task.State{
			TaskExecID: "child-exec-id",
			Status:     core.StatusFailed,
			Error:      childError,
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "child-task-id",
			},
		}

		result, err := handler.HandleSubtaskResponse(
			t.Context(),
			nil,
			childState,
			childConfig,
			task.StrategyWaitAll,
		)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "child-task-id", result.TaskID)
		assert.Equal(t, childError, result.Error)
		assert.Equal(t, core.StatusFailed, result.Status)
	})

	t.Run("Should handle different parallel strategies", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		childState := &task.State{
			TaskExecID: "child-exec-id",
			Status:     core.StatusSuccess,
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{ID: "child-task-id"},
		}

		strategies := []task.ParallelStrategy{
			task.StrategyWaitAll,
			task.StrategyFailFast,
			task.StrategyBestEffort,
			task.StrategyRace,
		}

		for _, strategy := range strategies {
			result, err := handler.HandleSubtaskResponse(
				t.Context(),
				nil,
				childState,
				childConfig,
				strategy,
			)

			assert.NoError(t, err, "Should handle strategy: %s", strategy)
			assert.NotNil(t, result)
			assert.Equal(t, "child-task-id", result.TaskID)
		}
	})
}

func TestParallelResponseHandler_ExtractParallelStrategy(t *testing.T) {
	t.Run("Should panic when calling deprecated method", func(t *testing.T) {
		handler := NewResponseHandler(nil, nil, nil)

		assert.PanicsWithValue(t,
			"ExtractParallelStrategy is deprecated. Use TaskConfigRepository.ExtractParallelStrategy instead",
			func() { handler.ExtractParallelStrategy(&task.State{}) })
	})
}

// Test struct field accessibility
func TestParallelResponseHandler_FieldAccess(t *testing.T) {
	t.Run("Should store and retrieve template engine", func(t *testing.T) {
		engine := &tplengine.TemplateEngine{}
		handler := NewResponseHandler(engine, nil, nil)

		assert.Same(t, engine, handler.templateEngine)
	})

	t.Run("Should store and retrieve context builder", func(t *testing.T) {
		builder := &shared.ContextBuilder{}
		handler := NewResponseHandler(nil, builder, nil)

		assert.Same(t, builder, handler.contextBuilder)
	})

	t.Run("Should store and retrieve base handler", func(t *testing.T) {
		base := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(nil, nil, base)

		assert.Same(t, base, handler.baseHandler)
	})
}

// Tests for HandleResponse method to increase coverage from 48.3% to 70%
func TestParallelResponseHandler_HandleResponse(t *testing.T) {
	t.Run("Should return validation error for nil input", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		// Act
		result, err := handler.HandleResponse(t.Context(), nil)
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
		result, err := handler.HandleResponse(t.Context(), input)
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
					Type: task.TaskTypeBasic, // Wrong type for parallel handler
				},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}
		// Act
		result, err := handler.HandleResponse(t.Context(), input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		var validationErr *shared.ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_type", validationErr.Field)
		assert.Contains(t, validationErr.Message, "handler type does not match task type")
	})
}

// Tests for ApplyDeferredOutputTransformation method to increase coverage
func TestParallelResponseHandler_ApplyDeferredOutputTransformation(t *testing.T) {
	t.Run("Should return validation error for nil input", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		// Act
		err := handler.ApplyDeferredOutputTransformation(t.Context(), nil)
		// Assert
		assert.Error(t, err)
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
		err := handler.ApplyDeferredOutputTransformation(t.Context(), input)
		// Assert
		assert.Error(t, err)
		var validationErr *shared.ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_config", validationErr.Field)
	})
	t.Run("Should handle valid input gracefully when deferred transformation is not applicable", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		baseHandler := &shared.BaseResponseHandler{}
		handler := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					Type: task.TaskTypeBasic, // Task type that should not defer transformation
				},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}
		// Act
		err := handler.ApplyDeferredOutputTransformation(t.Context(), input)
		// Assert - should return nil as basic tasks don't defer transformation
		assert.NoError(t, err)
	})
}
