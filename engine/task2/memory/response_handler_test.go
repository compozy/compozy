package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockParentStatusManager for testing
type MockParentStatusManager struct {
	mock.Mock
}

func (m *MockParentStatusManager) UpdateParentStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) error {
	args := m.Called(ctx, parentStateID, strategy)
	return args.Error(0)
}

func (m *MockParentStatusManager) GetAggregatedStatus(
	ctx context.Context,
	parentStateID core.ID,
	strategy task.ParallelStrategy,
) (core.StatusType, error) {
	args := m.Called(ctx, parentStateID, strategy)
	return args.Get(0).(core.StatusType), args.Error(1)
}

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

func TestResponseHandler_NewResponseHandler(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatYAML)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
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
		contextBuilder, err := shared.NewContextBuilder(t.Context())
		require.NoError(t, err)
		baseHandler := &shared.BaseResponseHandler{}
		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)
		assert.Equal(t, task.TaskTypeMemory, handler.Type())
	})
}

func TestResponseHandler_HandleResponse(t *testing.T) {
	templateEngine := tplengine.NewEngine(tplengine.FormatYAML)
	contextBuilder, err := shared.NewContextBuilder(t.Context())
	require.NoError(t, err)
	ctx := t.Context()

	t.Run("Should process valid memory task response", func(t *testing.T) {
		// Set up real repositories
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		// Set up mocks for other dependencies
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		// Create properly configured BaseResponseHandler with real repos
		baseHandler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)

		// Create a workflow state and save it
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusRunning,
			Input:          &core.Input{"test": "data"},
		}
		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a valid memory task state
		taskExecID := core.MustNewID()
		taskState := &task.State{
			TaskExecID:     taskExecID,
			TaskID:         "memory-task-1",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
			Output:         &core.Output{"key": "value"},
		}

		// Save initial task state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "memory-task-1",
					Type: task.TaskTypeMemory, // Correct type
					Outputs: &core.Input{
						"transformed": "{{ .output.key }}",
					},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: workflowState,
		}

		// Mock output transformer expectation
		outputTransformer.On("TransformOutput", ctx, taskState, input.TaskConfig, input.WorkflowConfig).
			Return(map[string]any{"transformed": "value"}, nil)

		// Act
		output, err := handler.HandleResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.NotNil(t, output.Response)
		assert.NotNil(t, output.State)
		assert.Equal(t, core.StatusSuccess, output.State.Status)

		// Verify the response is a MainTaskResponse
		mainTaskResp, ok := output.Response.(*task.MainTaskResponse)
		require.True(t, ok, "Response should be MainTaskResponse")
		assert.Equal(t, output.State, mainTaskResp.State)

		// Verify the state was persisted to the database
		savedState, err := taskRepo.GetState(ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		outputTransformer.AssertExpectations(t)
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
		// Set up real repositories
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		// Set up mocks for other dependencies
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		// Create properly configured BaseResponseHandler
		baseHandler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)

		// Create and save workflow state
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusRunning,
		}
		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		taskExecID := core.MustNewID()
		taskState := &task.State{
			TaskExecID:     taskExecID,
			TaskID:         "memory-task-fail",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}

		// Save initial task state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "memory-task-fail",
					Type: task.TaskTypeMemory, // Correct type
				},
			},
			TaskState:      taskState,
			ExecutionError: errors.New("simulated task execution error"),
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: workflowState,
		}

		// Act
		output, err := handler.HandleResponse(ctx, input)

		// Assert - should return error because no error transition is defined
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task failed with no error transition defined")
		assert.Contains(t, err.Error(), "simulated task execution error")
		assert.Nil(t, output)

		// Verify the state was persisted with failed status
		savedState, err := taskRepo.GetState(ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusFailed, savedState.Status)
		assert.NotNil(t, savedState.Error)
		assert.Contains(t, savedState.Error.Message, "simulated task execution error")
	})

	t.Run("Should handle error processing with error transition", func(t *testing.T) {
		// Set up real repositories
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		defer cleanup()

		// Set up mocks for other dependencies
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		// Create properly configured BaseResponseHandler
		baseHandler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		handler, err := NewResponseHandler(templateEngine, contextBuilder, baseHandler)
		require.NoError(t, err)

		// Create and save workflow state
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusRunning,
		}
		err = workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		taskExecID := core.MustNewID()
		taskState := &task.State{
			TaskExecID:     taskExecID,
			TaskID:         "memory-task-error",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}

		// Save initial task state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "memory-task-error",
					Type: task.TaskTypeMemory, // Correct type
					OnError: &core.ErrorTransition{
						Next: func() *string { s := "error-handler"; return &s }(),
					},
				},
			},
			TaskState:      taskState,
			ExecutionError: errors.New("task failed"),
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: workflowState,
		}

		// Act
		output, err := handler.HandleResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, core.StatusFailed, output.State.Status)
		assert.NotNil(t, output.State.Error)

		// Verify the response contains error transition
		mainTaskResp, ok := output.Response.(*task.MainTaskResponse)
		require.True(t, ok)
		assert.NotNil(t, mainTaskResp.OnError)
		assert.Equal(t, "error-handler", *mainTaskResp.OnError.Next)

		// Verify the state was persisted with failed status
		savedState, err := taskRepo.GetState(ctx, taskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusFailed, savedState.Status)
		assert.NotNil(t, savedState.Error)
	})
}
