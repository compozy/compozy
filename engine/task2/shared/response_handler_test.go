package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// MockOutputTransformer for testing
type MockOutputTransformer struct {
	mock.Mock
}

func (m *MockOutputTransformer) TransformOutput(
	ctx context.Context,
	state *task.State,
	config *task.Config,
	workflowConfig *workflow.Config,
) (map[string]any, error) {
	args := m.Called(ctx, state, config, workflowConfig)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]any), args.Error(1)
}

func TestNewBaseResponseHandler(t *testing.T) {
	t.Run("Should create handler with all dependencies", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		workflowRepo := &store.MockWorkflowRepo{}
		taskRepo := &store.MockTaskRepo{}
		outputTransformer := &MockOutputTransformer{}

		// Act
		handler := NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		// Assert
		assert.NotNil(t, handler)
		assert.Equal(t, templateEngine, handler.templateEngine)
		assert.Equal(t, contextBuilder, handler.contextBuilder)
		assert.Equal(t, parentStatusManager, handler.parentStatusManager)
		assert.Equal(t, workflowRepo, handler.workflowRepo)
		assert.Equal(t, taskRepo, handler.taskRepo)
		assert.Equal(t, outputTransformer, handler.outputTransformer)
	})
}

func TestProcessMainTaskResponse(t *testing.T) {
	t.Run("Should process successful task response", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		workflowRepo := &store.MockWorkflowRepo{}
		taskRepo := &store.MockTaskRepo{}
		outputTransformer := &MockOutputTransformer{}

		handler := NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		taskState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "test-task",
			Status:     core.StatusPending,
			Output:     &core.Output{"result": "success"},
		}

		input := &ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-task",
					Type: task.TaskTypeBasic,
					Outputs: &core.Input{
						"transformed": "{{ .result }}",
					},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: core.MustNewID(),
			},
		}

		// Mock expectations
		// Mock WithTx for saveTaskState
		taskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(pgx.Tx) error)
			fn(nil) // Simulate transaction execution
		}).Return(nil)

		taskRepo.On("GetStateForUpdate", ctx, mock.Anything, taskState.TaskExecID).Return(taskState, nil)
		taskRepo.On("UpsertStateWithTx", ctx, mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.Status == core.StatusSuccess
		})).Return(nil)

		outputTransformer.On("TransformOutput", ctx, taskState, input.TaskConfig, input.WorkflowConfig).
			Return(map[string]any{"transformed": "output"}, nil)

		// Act
		result, err := handler.ProcessMainTaskResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)
		assert.NotNil(t, result.State.Output)

		taskRepo.AssertExpectations(t)
		outputTransformer.AssertExpectations(t)
	})

	t.Run("Should handle task execution error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		workflowRepo := &store.MockWorkflowRepo{}
		taskRepo := &store.MockTaskRepo{}
		outputTransformer := &MockOutputTransformer{}

		handler := NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		executionError := errors.New("task execution failed")
		taskState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "test-task",
			Status:     core.StatusPending,
		}

		input := &ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-task",
					Type: task.TaskTypeBasic,
					OnError: &core.ErrorTransition{
						Next: func() *string { s := "error-task"; return &s }(),
					},
				},
			},
			TaskState:      taskState,
			ExecutionError: executionError,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: core.MustNewID(),
			},
		}

		// Mock expectations
		// Mock WithTx for saveTaskState
		taskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(pgx.Tx) error)
			fn(nil) // Simulate transaction execution
		}).Return(nil)

		taskRepo.On("GetStateForUpdate", ctx, mock.Anything, taskState.TaskExecID).Return(taskState, nil)
		taskRepo.On("UpsertStateWithTx", ctx, mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.Status == core.StatusFailed && state.Error != nil
		})).Return(nil)

		// Act
		result, err := handler.ProcessMainTaskResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusFailed, result.State.Status)
		assert.NotNil(t, result.State.Error)
		assert.Equal(t, executionError.Error(), result.State.Error.Message)

		taskRepo.AssertExpectations(t)
	})

	t.Run("Should handle context cancellation", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(context.Background())

		taskRepo := &store.MockTaskRepo{}

		handler := NewBaseResponseHandler(nil, nil, nil, nil, taskRepo, nil)

		input := &ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{ID: "test-task"},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID()},
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  &workflow.State{WorkflowID: "test-workflow"},
		}

		// Mock WithTx to return context canceled error
		taskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).Return(context.Canceled)

		// Cancel the context before processing
		cancel()

		// Act
		result, err := handler.ProcessMainTaskResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, input.TaskState, result.State)
	})
}

func TestShouldDeferOutputTransformation(t *testing.T) {
	t.Run("Should defer for collection tasks", func(t *testing.T) {
		handler := &BaseResponseHandler{}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeCollection,
			},
		}
		assert.True(t, handler.ShouldDeferOutputTransformation(config))
	})

	t.Run("Should defer for parallel tasks", func(t *testing.T) {
		handler := &BaseResponseHandler{}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeParallel,
			},
		}
		assert.True(t, handler.ShouldDeferOutputTransformation(config))
	})

	t.Run("Should not defer for basic tasks", func(t *testing.T) {
		handler := &BaseResponseHandler{}
		config := &task.Config{
			BaseConfig: task.BaseConfig{
				Type: task.TaskTypeBasic,
			},
		}
		assert.False(t, handler.ShouldDeferOutputTransformation(config))
	})
}

func TestApplyDeferredOutputTransformation(t *testing.T) {
	t.Run("Should apply deferred transformation successfully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		workflowRepo := &store.MockWorkflowRepo{}
		taskRepo := &store.MockTaskRepo{}
		outputTransformer := &MockOutputTransformer{}

		handler := NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		taskExecID := core.MustNewID()
		taskState := &task.State{
			TaskExecID: taskExecID,
			TaskID:     "test-collection",
			Status:     core.StatusSuccess,
			Output:     &core.Output{"items": []string{"a", "b", "c"}},
		}

		input := &ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-collection",
					Type:    task.TaskTypeCollection,
					Outputs: &core.Input{"count": "{{ len .output.items }}"},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: core.MustNewID(),
			},
		}

		// Mock expectations
		// Mock WithTx for ApplyDeferredOutputTransformation
		taskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(pgx.Tx) error)
			fn(nil) // Simulate transaction execution
		}).Return(nil)

		taskRepo.On("GetStateForUpdate", ctx, mock.Anything, taskState.TaskExecID).Return(taskState, nil)

		outputTransformer.On("TransformOutput", ctx, taskState, input.TaskConfig, input.WorkflowConfig).
			Return(map[string]any{"count": 3}, nil)

		taskRepo.On("UpsertStateWithTx", ctx, mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.Output != nil
		})).Return(nil)

		// Act
		err := handler.ApplyDeferredOutputTransformation(ctx, input)

		// Assert
		require.NoError(t, err)
		taskRepo.AssertExpectations(t)
		outputTransformer.AssertExpectations(t)
	})

	t.Run("Should handle transformation failure", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		workflowRepo := &store.MockWorkflowRepo{}
		taskRepo := &store.MockTaskRepo{}
		outputTransformer := &MockOutputTransformer{}

		handler := NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		taskExecID := core.MustNewID()
		taskState := &task.State{
			TaskExecID: taskExecID,
			TaskID:     "test-collection",
			Status:     core.StatusSuccess,
			Output:     &core.Output{"items": []string{"a", "b", "c"}},
		}

		input := &ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-collection",
					Type:    task.TaskTypeCollection,
					Outputs: &core.Input{"count": "{{ .invalid.path }}"},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: core.MustNewID(),
			},
		}

		transformError := errors.New("transformation failed")

		// Mock expectations
		// Mock WithTx for ApplyDeferredOutputTransformation
		taskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(pgx.Tx) error)
			fn(nil) // Simulate transaction execution
		}).Return(nil)

		taskRepo.On("GetStateForUpdate", ctx, mock.Anything, taskState.TaskExecID).Return(taskState, nil)

		outputTransformer.On("TransformOutput", ctx, taskState, input.TaskConfig, input.WorkflowConfig).
			Return(nil, transformError)

		taskRepo.On("UpsertStateWithTx", ctx, mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.Status == core.StatusFailed && state.Error != nil
		})).Return(nil)

		// Act
		err := handler.ApplyDeferredOutputTransformation(ctx, input)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output transformation failed")
		taskRepo.AssertExpectations(t)
		outputTransformer.AssertExpectations(t)
	})
}

func TestValidateInput(t *testing.T) {
	handler := &BaseResponseHandler{}

	t.Run("Should validate nil input", func(t *testing.T) {
		err := handler.ValidateInput(nil)
		require.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
		assert.Contains(t, err.Error(), "input cannot be nil")
	})

	t.Run("Should validate nil task config", func(t *testing.T) {
		input := &ResponseInput{
			TaskState:      &task.State{},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}
		err := handler.ValidateInput(input)
		require.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
		assert.Contains(t, err.Error(), "task config cannot be nil")
	})

	t.Run("Should validate nil task state", func(t *testing.T) {
		input := &ResponseInput{
			TaskConfig:     &task.Config{},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}
		err := handler.ValidateInput(input)
		require.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
		assert.Contains(t, err.Error(), "task state cannot be nil")
	})

	t.Run("Should validate nil workflow config", func(t *testing.T) {
		input := &ResponseInput{
			TaskConfig:    &task.Config{},
			TaskState:     &task.State{},
			WorkflowState: &workflow.State{},
		}
		err := handler.ValidateInput(input)
		require.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
		assert.Contains(t, err.Error(), "workflow config cannot be nil")
	})

	t.Run("Should validate nil workflow state", func(t *testing.T) {
		input := &ResponseInput{
			TaskConfig:     &task.Config{},
			TaskState:      &task.State{},
			WorkflowConfig: &workflow.Config{},
		}
		err := handler.ValidateInput(input)
		require.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
		assert.Contains(t, err.Error(), "workflow state cannot be nil")
	})

	t.Run("Should pass validation with all required fields", func(t *testing.T) {
		input := &ResponseInput{
			TaskConfig:     &task.Config{},
			TaskState:      &task.State{},
			WorkflowConfig: &workflow.Config{},
			WorkflowState:  &workflow.State{},
		}
		err := handler.ValidateInput(input)
		require.NoError(t, err)
	})
}

func TestProcessTransitions(t *testing.T) {
	t.Run("Should process success transitions", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &ContextBuilder{}
		handler := &BaseResponseHandler{
			templateEngine: templateEngine,
			contextBuilder: contextBuilder,
		}

		successTransition := &core.SuccessTransition{
			Next: func() *string { s := "next-task"; return &s }(),
		}

		input := &ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:        "test-task",
					OnSuccess: successTransition,
				},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID()},
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  &workflow.State{WorkflowID: "test-workflow"},
		}

		// Act
		onSuccess, onError, err := handler.processTransitions(ctx, input, true, nil)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, onSuccess)
		assert.Equal(t, "next-task", *onSuccess.Next)
		assert.Nil(t, onError)
	})

	t.Run("Should process error transitions", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &ContextBuilder{}
		handler := &BaseResponseHandler{
			templateEngine: templateEngine,
			contextBuilder: contextBuilder,
		}

		errorTransition := &core.ErrorTransition{
			Next: func() *string { s := "error-handler"; return &s }(),
		}

		input := &ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-task",
					OnError: errorTransition,
				},
			},
			TaskState:      &task.State{TaskExecID: core.MustNewID()},
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  &workflow.State{WorkflowID: "test-workflow"},
		}

		executionErr := errors.New("task failed")

		// Act
		onSuccess, onError, err := handler.processTransitions(ctx, input, false, executionErr)

		// Assert
		require.NoError(t, err)
		assert.Nil(t, onSuccess)
		assert.NotNil(t, onError)
		assert.Equal(t, "error-handler", *onError.Next)
	})
}

func TestUpdateParentStatusIfNeeded(t *testing.T) {
	t.Run("Should skip update for non-child tasks", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		parentStatusManager := &MockParentStatusManager{}
		handler := &BaseResponseHandler{
			parentStatusManager: parentStatusManager,
		}

		state := &task.State{
			TaskExecID: core.MustNewID(),
			// No ParentStateID - this is not a child task
		}

		// Act
		err := handler.updateParentStatusIfNeeded(ctx, state)

		// Assert
		require.NoError(t, err)
		parentStatusManager.AssertNotCalled(t, "UpdateParentStatus")
	})

	t.Run("Should update parent status for child tasks", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		parentStatusManager := &MockParentStatusManager{}
		taskRepo := &store.MockTaskRepo{}
		handler := &BaseResponseHandler{
			parentStatusManager: parentStatusManager,
			taskRepo:            taskRepo,
		}

		parentID := core.MustNewID()
		state := &task.State{
			TaskExecID:    core.MustNewID(),
			ParentStateID: &parentID,
		}

		// Mock getting parent state to extract strategy
		parentState := &task.State{
			TaskExecID: parentID,
			Input:      &core.Input{"strategy": "wait_all"},
		}
		taskRepo.On("GetState", ctx, parentID).Return(parentState, nil)

		parentStatusManager.On("UpdateParentStatus", ctx, parentID, task.StrategyWaitAll).
			Return(nil)

		// Act
		err := handler.updateParentStatusIfNeeded(ctx, state)

		// Assert
		require.NoError(t, err)
		parentStatusManager.AssertExpectations(t)
	})
}
