package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/compozy/compozy/test/integration/worker/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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

func TestBaseResponseHandler_TransactionSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should handle transaction safety with real database", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() {
			dbHelper.Cleanup(t)
		})

		ctx := context.Background()
		pool := dbHelper.GetPool()

		// Create real repository instances
		taskRepo := store.NewTaskRepo(pool)
		workflowRepo := store.NewWorkflowRepo(pool)

		// Create handler with real dependencies
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		handler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		// Create workflow state first (required by foreign key constraint)
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a task state
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-task",
			Status:         core.StatusPending,
			Output:         &core.Output{"data": "test-output"},
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-task",
					Type:    task.TaskTypeBasic,
					Outputs: &core.Input{"result": "{{ .output.data }}"},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow-state",
			},
		}

		// Mock expectations
		outputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			map[string]any{"result": "transformed"}, nil)

		// Act - process the response
		result, err := handler.ProcessMainTaskResponse(ctx, input)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, core.StatusSuccess, taskState.Status)

		// Verify state was saved with transaction safety
		savedState, err := taskRepo.GetState(ctx, taskState.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		outputTransformer.AssertExpectations(t)
	})
}

func TestBaseResponseHandler_DeferredOutputTransformation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should handle deferred output transformation with real database", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() {
			dbHelper.Cleanup(t)
		})

		ctx := context.Background()
		pool := dbHelper.GetPool()

		// Create real repository instances
		taskRepo := store.NewTaskRepo(pool)
		workflowRepo := store.NewWorkflowRepo(pool)

		// Create handler with real dependencies
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		handler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		// Create workflow state first (required by foreign key constraint)
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a collection task state
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-collection-task",
			Status:         core.StatusSuccess,
			Output:         &core.Output{"items": []string{"item1", "item2"}},
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input for deferred transformation
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-collection-task",
					Type:    task.TaskTypeCollection,
					Outputs: &core.Input{"result": "{{ .output.items }}"},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
		}

		// Mock expectations
		outputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			map[string]any{"result": []string{"item1", "item2"}}, nil)

		// Act - apply deferred transformation
		err = handler.ApplyDeferredOutputTransformation(ctx, input)

		// Assert
		require.NoError(t, err)

		// Verify state was updated with transformed output
		savedState, err := taskRepo.GetState(ctx, taskState.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.NotNil(t, savedState.Output)

		outputTransformer.AssertExpectations(t)
	})
}

func TestBaseResponseHandler_TransformationFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should handle transformation failure with proper rollback", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() {
			dbHelper.Cleanup(t)
		})

		ctx := context.Background()
		pool := dbHelper.GetPool()

		// Create real repository instances
		taskRepo := store.NewTaskRepo(pool)
		workflowRepo := store.NewWorkflowRepo(pool)

		// Create handler with real dependencies
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		handler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		// Create workflow state first (required by foreign key constraint)
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a task state
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-failure-task",
			Status:         core.StatusSuccess,
			Output:         &core.Output{"data": "test"},
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input for deferred transformation
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:      "test-failure-task",
					Type:    task.TaskTypeCollection,
					Outputs: &core.Input{"result": "{{ .invalid }}"},
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: &workflow.State{
				WorkflowID:     "test-workflow",
				WorkflowExecID: workflowExecID,
			},
		}

		// Mock transformation failure
		transformErr := errors.New("transformation failed")
		outputTransformer.On("TransformOutput", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			nil, transformErr)

		// Act - apply deferred transformation (should fail)
		err = handler.ApplyDeferredOutputTransformation(ctx, input)

		// Assert - should return the transformation error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "transformation failed")

		// Verify state was NOT changed due to transaction rollback
		// This is the correct behavior - if transformation fails, the entire transaction
		// should roll back, leaving the state unchanged
		savedState, err := taskRepo.GetState(ctx, taskState.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
		assert.Nil(t, savedState.Error)

		outputTransformer.AssertExpectations(t)
	})
}

func TestBaseResponseHandler_ConcurrentSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	t.Run("Should handle concurrent modifications gracefully", func(t *testing.T) {
		t.Parallel()

		// Setup test infrastructure
		dbHelper := helpers.NewDatabaseHelper(t)
		t.Cleanup(func() {
			dbHelper.Cleanup(t)
		})

		ctx := context.Background()
		pool := dbHelper.GetPool()

		// Create real repository instances
		taskRepo := store.NewTaskRepo(pool)
		workflowRepo := store.NewWorkflowRepo(pool)

		// Create handler with real dependencies
		templateEngine := &tplengine.TemplateEngine{}
		contextBuilder := &shared.ContextBuilder{}
		parentStatusManager := &MockParentStatusManager{}
		outputTransformer := &MockOutputTransformer{}

		handler := shared.NewBaseResponseHandler(
			templateEngine,
			contextBuilder,
			parentStatusManager,
			workflowRepo,
			taskRepo,
			outputTransformer,
		)

		// Create workflow state first (required by foreign key constraint)
		workflowExecID := core.MustNewID()
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		err := workflowRepo.UpsertState(ctx, workflowState)
		require.NoError(t, err)

		// Create a task state
		taskState := &task.State{
			TaskExecID:     core.MustNewID(),
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			TaskID:         "test-concurrent-task",
			Status:         core.StatusPending,
			Output:         &core.Output{"data": "initial"},
		}

		// Save initial state
		err = taskRepo.UpsertState(ctx, taskState)
		require.NoError(t, err)

		// Prepare input
		input := &shared.ResponseInput{
			TaskConfig: &task.Config{
				BaseConfig: task.BaseConfig{
					ID:   "test-concurrent-task",
					Type: task.TaskTypeBasic,
				},
			},
			TaskState: taskState,
			WorkflowConfig: &workflow.Config{
				ID: "test-workflow",
			},
			WorkflowState: &workflow.State{
				WorkflowID: "test-workflow-state",
			},
		}

		// Simulate concurrent modification by updating the task state externally
		taskState.Status = core.StatusSuccess
		taskState.Output = &core.Output{"data": "updated"}

		// Act - process the response (should handle concurrent updates gracefully)
		result, err := handler.ProcessMainTaskResponse(ctx, input)

		// Assert - should complete successfully despite concurrent modification
		require.NoError(t, err)
		assert.NotNil(t, result)

		// Verify final state reflects the handler's changes
		savedState, err := taskRepo.GetState(ctx, taskState.TaskExecID)
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, savedState.Status)
	})
}
