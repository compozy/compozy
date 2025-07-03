package router

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/router"
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

func TestRouterResponseHandler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should process router task response with routing decision", func(t *testing.T) {
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

		// Create router response handler
		handler := router.NewResponseHandler(templateEngine, contextBuilder, baseHandler)

		// Create workflow state first
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a router task state with routing decision
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-router-task",
			Status:         core.StatusPending,
			Output:         &core.Output{"route": "path-a"},
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input with router configuration
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-router-task",
					Type:    task.TaskTypeRouter,
					Outputs: &core.Input{"decision": "{{ .output.route }}"},
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Mock output transformation
		outputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(map[string]any{"decision": "path-a"}, nil)

		// Act - process the response
		result, err := handler.HandleResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)

		// Verify it's a MainTaskResponse with routing info
		mainTaskResp, ok := result.Response.(*task.MainTaskResponse)
		require.True(t, ok)
		assert.Equal(t, taskState, mainTaskResp.State)

		// Verify state was saved to database
		savedState, err := taskRepo.GetState(ctx, taskState.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		outputTransformer.AssertExpectations(t)
	})

	t.Run("Should validate routing decision output", func(t *testing.T) {
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

		handler := router.NewResponseHandler(templateEngine, contextBuilder, baseHandler)

		// Create workflow state
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a router task state with invalid output
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-invalid-router",
			Status:         core.StatusPending,
			Output:         &core.Output{}, // Empty output - invalid for router
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input with router configuration
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-invalid-router",
					Type: task.TaskTypeRouter,
				},
			},
			TaskState:      taskState,
			WorkflowConfig: &workflow.Config{ID: "test-workflow"},
			WorkflowState:  workflowState,
		}

		// Act - process the response
		result, err := handler.HandleResponse(ctx, input)

		// Assert - should still succeed but with appropriate handling
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, result.State.Status)
	})
}

func TestRouterResponseHandler_Type(t *testing.T) {
	t.Run("Should return router task type", func(t *testing.T) {
		handler := router.NewResponseHandler(nil, nil, nil)
		assert.Equal(t, task.TaskTypeRouter, handler.Type())
	})
}
