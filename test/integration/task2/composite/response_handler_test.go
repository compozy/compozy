package composite

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/composite"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockOutputTransformer implements OutputTransformer for testing
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

// MockParentStatusManager implements ParentStatusManager for testing
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

func TestCompositeResponseHandler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should process composite task response", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ctx := context.Background()
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		t.Cleanup(cleanup)

		// Create handler dependencies
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &shared.ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		// Create base handler with real repositories
		baseHandler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		// Create composite response handler
		handler := composite.NewResponseHandler(templateEngine, contextBuilder, baseHandler)

		// Create workflow state first
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a composite task state
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-composite-task",
			Status:         core.StatusPending,
			Output:         &core.Output{"step": "completed"},
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input with composite configuration
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-composite-task",
					Type:    task.TaskTypeComposite,
					Outputs: &core.Input{"result": "{{ .output.step }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Mock output transformation
		outputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"result": "completed"}, nil)

		// Act - process the response
		result, err := handler.HandleResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)

		// Verify state was saved to database
		savedState, err := taskRepo.GetState(ctx, taskState.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		outputTransformer.AssertExpectations(t)
	})

	t.Run("Should handle subtask response for sequential execution", func(t *testing.T) {
		// Arrange
		handler := composite.NewResponseHandler(nil, nil, nil)

		parentState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "parent-composite",
		}

		childState := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "step-1",
			Status:     core.StatusSuccess,
			Output:     &core.Output{"stepResult": "step1-complete"},
		}

		childConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID: "step-1",
			},
		}

		// Act
		response, err := handler.HandleSubtaskResponse(context.Background(), parentState, childState, childConfig)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "step-1", response.TaskID)
		assert.Equal(t, core.StatusSuccess, response.Status)
		assert.Equal(t, childState.Output, response.Output)
	})

	t.Run("Should handle failed composite task", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		ctx := context.Background()
		taskRepo, workflowRepo, cleanup := utils.SetupTestRepos(ctx, t)
		t.Cleanup(cleanup)

		// Create handler dependencies
		templateEngine := tplengine.NewEngine(tplengine.FormatJSON)
		contextBuilder := &shared.ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		baseHandler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		handler := composite.NewResponseHandler(templateEngine, contextBuilder, baseHandler)

		// Create workflow state
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a failed composite task state
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-failed-composite",
			Status:         core.StatusPending,
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input with execution error
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-failed-composite",
					Type: task.TaskTypeComposite,
					OnError: &core.ErrorTransition{
						Next: func() *string { s := "error-handler"; return &s }(),
					},
				},
			},
			TaskState:      taskState,
			ExecutionError: assert.AnError,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Act - process the response
		result, err := handler.HandleResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusFailed, result.State.Status)
		assert.NotNil(t, result.State.Error)

		// Verify state was saved to database
		savedState, err := taskRepo.GetState(ctx, taskState.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusFailed, savedState.Status)
		assert.NotNil(t, savedState.Error)
	})
}

func TestCompositeResponseHandler_Type(t *testing.T) {
	t.Run("Should return composite task type", func(t *testing.T) {
		handler := composite.NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeComposite, handler.Type())
	})
}
