package uc

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleResponse_ShouldUpdateParentStatus(t *testing.T) {
	t.Run("Should not update when status is the same", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)
		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

		result := handleResponse.parentStatusUpdater.ShouldUpdateParentStatus(core.StatusRunning, core.StatusRunning)
		assert.False(t, result)
	})

	t.Run("Should update when transitioning to success", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)
		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

		result := handleResponse.parentStatusUpdater.ShouldUpdateParentStatus(core.StatusRunning, core.StatusSuccess)
		assert.True(t, result)
	})

	t.Run("Should update when transitioning to failed", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)
		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

		result := handleResponse.parentStatusUpdater.ShouldUpdateParentStatus(core.StatusRunning, core.StatusFailed)
		assert.True(t, result)
	})

	t.Run("Should update when transitioning from pending to running", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)
		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

		result := handleResponse.parentStatusUpdater.ShouldUpdateParentStatus(core.StatusPending, core.StatusRunning)
		assert.True(t, result)
	})

	t.Run("Should update when transitioning from success to failed", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)
		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

		result := handleResponse.parentStatusUpdater.ShouldUpdateParentStatus(core.StatusSuccess, core.StatusFailed)
		assert.True(t, result)
	})

	t.Run("Should not update when transitioning from terminal to active", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)
		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

		result := handleResponse.parentStatusUpdater.ShouldUpdateParentStatus(core.StatusSuccess, core.StatusRunning)
		assert.False(t, result)
	})
}

func TestHandleResponse_UpdateParentStatusIfNeeded(t *testing.T) {
	t.Run("Should not update when child task has no parent", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)
		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)

		childState := &task.State{
			TaskExecID:    core.ID("child-123"),
			TaskID:        "child-task",
			Status:        core.StatusSuccess,
			ParentStateID: nil,
		}

		ctx := context.Background()
		err := handleResponse.updateParentStatusIfNeeded(ctx, childState)

		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
		mockWorkflowRepo.AssertExpectations(t)
	})

	t.Run("Should not update when parent task is not parallel", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)

		parentID := core.ID("parent-456")
		childState := &task.State{
			TaskExecID:    core.ID("child-123"),
			TaskID:        "child-task",
			Status:        core.StatusSuccess,
			ParentStateID: &parentID,
		}
		parentState := &task.State{
			TaskExecID:    core.ID("parent-456"),
			TaskID:        "parent-task",
			Status:        core.StatusRunning,
			ExecutionType: task.ExecutionBasic,
		}

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)

		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)
		ctx := context.Background()
		err := handleResponse.updateParentStatusIfNeeded(ctx, childState)

		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
		mockWorkflowRepo.AssertExpectations(t)
	})

	t.Run("Should succeed when wait_all strategy has all children complete", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)

		parentID := core.ID("parent-456")
		childState := &task.State{
			TaskExecID:    core.ID("child-123"),
			TaskID:        "child-task",
			Status:        core.StatusSuccess,
			ParentStateID: &parentID,
		}
		parentState := &task.State{
			TaskExecID:    core.ID("parent-456"),
			TaskID:        "parent-task",
			Status:        core.StatusRunning,
			ExecutionType: task.ExecutionParallel,
			Input: &core.Input{
				"_parallel_config": map[string]any{
					"strategy": string(task.StrategyWaitAll),
				},
			},
		}
		progressInfo := &task.ProgressInfo{
			TotalChildren:  2,
			CompletedCount: 2,
			FailedCount:    0,
			RunningCount:   0,
			PendingCount:   0,
		}

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)
		mockTaskRepo.On("GetProgressInfo", mock.Anything, parentState.TaskExecID).Return(progressInfo, nil)

		// Mock the transaction-related calls
		mockTaskRepo.On("WithTx", mock.Anything, mock.AnythingOfType("func(pgx.Tx) error")).
			Return(nil).
			Run(func(args mock.Arguments) {
				// Execute the transaction function with a nil transaction for testing
				fn := args.Get(1).(func(pgx.Tx) error)
				fn(nil)
			})
		mockTaskRepo.On("GetStateForUpdate", mock.Anything, mock.Anything, parentState.TaskExecID).
			Return(parentState, nil)
		mockTaskRepo.On("UpsertStateWithTx", mock.Anything, mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.TaskExecID == parentState.TaskExecID && state.Status == core.StatusSuccess
		})).Return(nil)

		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)
		ctx := context.Background()
		err := handleResponse.updateParentStatusIfNeeded(ctx, childState)

		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
		mockWorkflowRepo.AssertExpectations(t)
	})

	t.Run("Should fail when fail_fast strategy has one failure", func(t *testing.T) {
		mockWorkflowRepo := new(store.MockWorkflowRepo)
		mockTaskRepo := new(store.MockTaskRepo)

		parentID := core.ID("parent-456")
		childState := &task.State{
			TaskExecID:    core.ID("child-123"),
			TaskID:        "child-task",
			Status:        core.StatusFailed,
			ParentStateID: &parentID,
		}
		parentState := &task.State{
			TaskExecID:    core.ID("parent-456"),
			TaskID:        "parent-task",
			Status:        core.StatusRunning,
			ExecutionType: task.ExecutionParallel,
			Input: &core.Input{
				"_parallel_config": map[string]any{
					"strategy": string(task.StrategyFailFast),
				},
			},
		}
		progressInfo := &task.ProgressInfo{
			TotalChildren:  2,
			CompletedCount: 0,
			FailedCount:    1,
			RunningCount:   1,
			PendingCount:   0,
		}

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)
		mockTaskRepo.On("GetProgressInfo", mock.Anything, parentState.TaskExecID).Return(progressInfo, nil)

		// Mock the transaction-related calls
		mockTaskRepo.On("WithTx", mock.Anything, mock.AnythingOfType("func(pgx.Tx) error")).
			Return(nil).
			Run(func(args mock.Arguments) {
				// Execute the transaction function with a nil transaction for testing
				fn := args.Get(1).(func(pgx.Tx) error)
				fn(nil)
			})
		mockTaskRepo.On("GetStateForUpdate", mock.Anything, mock.Anything, parentState.TaskExecID).
			Return(parentState, nil)
		mockTaskRepo.On("UpsertStateWithTx", mock.Anything, mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.TaskExecID == parentState.TaskExecID && state.Status == core.StatusFailed
		})).Return(nil)

		handleResponse := NewHandleResponse(mockWorkflowRepo, mockTaskRepo)
		ctx := context.Background()
		err := handleResponse.updateParentStatusIfNeeded(ctx, childState)

		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
		mockWorkflowRepo.AssertExpectations(t)
	})
}
