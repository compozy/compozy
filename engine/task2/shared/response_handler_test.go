package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

func taskConfigBasic() *task.Config {
	config := &task.Config{}
	config.Type = task.TaskTypeBasic
	return config
}

func TestNewBaseResponseHandler(t *testing.T) {
	t.Run("Should create handler with all dependencies", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		workflowRepo := &store.MockWorkflowRepo{}
		taskRepo := &store.MockTaskRepo{}

		// Act
		handler := NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
		)

		// Assert
		assert.NotNil(t, handler)
		assert.Equal(t, templateEngine, handler.templateEngine)
		assert.Equal(t, contextBuilder, handler.contextBuilder)
		assert.Equal(t, parentStatusManager, handler.parentStatusManager)
		assert.Equal(t, workflowRepo, handler.workflowRepo)
		assert.Equal(t, taskRepo, handler.taskRepo)
	})
}

func TestBaseResponseHandler_ProcessMainTaskResponse(t *testing.T) {
	t.Run("Should process successful task execution", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		mockParentStatusManager := new(MockParentStatusManager)

		handler := NewBaseResponseHandler(
			nil, nil, mockParentStatusManager, nil, mockTaskRepo,
		)

		taskState := &task.State{
			TaskExecID: core.MustNewID(),
			Status:     core.StatusRunning,
		}

		config := taskConfigBasic()
		config.Type = task.TaskTypeBasic
		input := &ResponseInput{
			TaskConfig:     config,
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		mockTaskRepo.On("UpsertState", mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.Status == core.StatusSuccess
		})).Return(nil)

		// Act
		output, err := handler.ProcessMainTaskResponse(context.Background(), input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, core.StatusSuccess, output.State.Status)
		assert.NotNil(t, output.Response)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should process failed task execution", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		mockParentStatusManager := new(MockParentStatusManager)

		handler := NewBaseResponseHandler(
			nil, nil, mockParentStatusManager, nil, mockTaskRepo,
		)

		taskState := &task.State{
			TaskExecID: core.MustNewID(),
			Status:     core.StatusRunning,
		}

		executionError := errors.New("execution failed")
		config := taskConfigBasic()
		config.Type = task.TaskTypeBasic
		input := &ResponseInput{
			TaskConfig:     config,
			TaskState:      taskState,
			ExecutionError: executionError,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		mockTaskRepo.On("UpsertState", mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.Status == core.StatusFailed && state.Error != nil && state.Error.Message == "execution failed"
		})).Return(nil)

		// Act
		output, err := handler.ProcessMainTaskResponse(context.Background(), input)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusFailed, output.State.Status)
		assert.NotNil(t, output.State.Error)
		assert.Equal(t, "execution failed", output.State.Error.Message)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should update parent status for child task", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		mockParentStatusManager := new(MockParentStatusManager)

		handler := NewBaseResponseHandler(
			nil, nil, mockParentStatusManager, nil, mockTaskRepo,
		)

		parentID := core.MustNewID()
		taskState := &task.State{
			TaskExecID:    core.MustNewID(),
			Status:        core.StatusRunning,
			ParentStateID: &parentID,
		}

		config := taskConfigBasic()
		config.Type = task.TaskTypeBasic
		input := &ResponseInput{
			TaskConfig:     config,
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		mockTaskRepo.On("UpsertState", mock.Anything, mock.Anything).Return(nil)
		mockParentStatusManager.On("UpdateParentStatus", mock.Anything, parentID, task.StrategyWaitAll).Return(nil)

		// Act
		output, err := handler.ProcessMainTaskResponse(context.Background(), input)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, output.State.Status)
		mockTaskRepo.AssertExpectations(t)
		mockParentStatusManager.AssertExpectations(t)
	})

	t.Run("Should handle context cancellation gracefully", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		handler := NewBaseResponseHandler(nil, nil, nil, nil, mockTaskRepo)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		taskState := &task.State{TaskExecID: core.MustNewID()}
		input := &ResponseInput{
			TaskConfig:     taskConfigBasic(),
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Act
		output, err := handler.ProcessMainTaskResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, taskState, output.State)
	})

	t.Run("Should handle task state save error", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		handler := NewBaseResponseHandler(nil, nil, nil, nil, mockTaskRepo)

		saveError := errors.New("database error")
		mockTaskRepo.On("UpsertState", mock.Anything, mock.Anything).Return(saveError)

		input := &ResponseInput{
			TaskConfig:     taskConfigBasic(),
			TaskState:      &task.State{TaskExecID: core.MustNewID()},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Act
		output, err := handler.ProcessMainTaskResponse(context.Background(), input)

		// Assert
		require.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "failed to save task state")
		mockTaskRepo.AssertExpectations(t)
	})
}

func TestBaseResponseHandler_ShouldDeferOutputTransformation(t *testing.T) {
	t.Run("Should defer output transformation for collection tasks", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		config := &task.Config{}
		config.Type = task.TaskTypeCollection

		// Act
		shouldDefer := handler.ShouldDeferOutputTransformation(config)

		// Assert
		assert.True(t, shouldDefer)
	})

	t.Run("Should defer output transformation for parallel tasks", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		config := &task.Config{}
		config.Type = task.TaskTypeParallel

		// Act
		shouldDefer := handler.ShouldDeferOutputTransformation(config)

		// Assert
		assert.True(t, shouldDefer)
	})

	t.Run("Should not defer output transformation for basic tasks", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		config := taskConfigBasic()

		// Act
		shouldDefer := handler.ShouldDeferOutputTransformation(config)

		// Assert
		assert.False(t, shouldDefer)
	})

	t.Run("Should not defer output transformation for composite tasks", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		config := &task.Config{}
		config.Type = task.TaskTypeComposite

		// Act
		shouldDefer := handler.ShouldDeferOutputTransformation(config)

		// Assert
		assert.False(t, shouldDefer)
	})
}

func TestBaseResponseHandler_ValidateInput(t *testing.T) {
	t.Run("Should pass validation for valid input", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		input := &ResponseInput{
			TaskConfig:     taskConfigBasic(),
			TaskState:      &task.State{},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Act
		err := handler.ValidateInput(input)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should fail validation for nil input", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)

		// Act
		err := handler.ValidateInput(nil)

		// Assert
		require.Error(t, err)
		var validationErr *ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "input", validationErr.Field)
	})

	t.Run("Should fail validation for nil task config", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		input := &ResponseInput{
			TaskState:      &task.State{},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Act
		err := handler.ValidateInput(input)

		// Assert
		require.Error(t, err)
		var validationErr *ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_config", validationErr.Field)
	})

	t.Run("Should fail validation for nil task state", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		input := &ResponseInput{
			TaskConfig:     taskConfigBasic(),
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}

		// Act
		err := handler.ValidateInput(input)

		// Assert
		require.Error(t, err)
		var validationErr *ValidationError
		assert.ErrorAs(t, err, &validationErr)
		assert.Equal(t, "task_state", validationErr.Field)
	})
}

func TestBaseResponseHandler_CreateResponseContext(t *testing.T) {
	t.Run("Should create context for child task", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		parentID := core.MustNewID()
		input := &ResponseInput{
			TaskConfig: taskConfigBasic(),
			TaskState: &task.State{
				ParentStateID: &parentID,
			},
		}

		// Act
		context := handler.CreateResponseContext(input)

		// Assert
		assert.True(t, context.IsParentTask)
		assert.Equal(t, parentID.String(), context.ParentTaskID)
		assert.NotNil(t, context.DeferredConfig)
		assert.False(t, context.DeferredConfig.ShouldDefer)
	})

	t.Run("Should create context for parent task", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)
		config := &task.Config{}
		config.Type = task.TaskTypeCollection
		input := &ResponseInput{
			TaskConfig: config,
			TaskState:  &task.State{},
		}

		// Act
		context := handler.CreateResponseContext(input)

		// Assert
		assert.False(t, context.IsParentTask)
		assert.Empty(t, context.ParentTaskID)
		assert.NotNil(t, context.DeferredConfig)
		assert.True(t, context.DeferredConfig.ShouldDefer)
	})
}

func TestBaseResponseHandler_CreateDeferredOutputConfig(t *testing.T) {
	t.Run("Should create deferred config for collection tasks", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)

		// Act
		config := handler.CreateDeferredOutputConfig(task.TaskTypeCollection, "test reason")

		// Assert
		assert.True(t, config.ShouldDefer)
		assert.Equal(t, "test reason", config.Reason)
	})

	t.Run("Should create non-deferred config for basic tasks", func(t *testing.T) {
		// Arrange
		handler := NewBaseResponseHandler(nil, nil, nil, nil, nil)

		// Act
		config := handler.CreateDeferredOutputConfig(task.TaskTypeBasic, "test reason")

		// Assert
		assert.False(t, config.ShouldDefer)
		assert.Equal(t, "test reason", config.Reason)
	})
}
