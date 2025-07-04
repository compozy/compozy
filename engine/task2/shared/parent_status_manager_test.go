package shared

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
)

func TestDefaultParentStatusManager_UpdateParentStatus(t *testing.T) {
	t.Run("Should update parent status based on all children success", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockRepo)
		ctx := context.Background()
		parentID := core.MustNewID()

		// Parent state
		parentState := &task.State{
			TaskExecID: parentID,
			TaskID:     "parent-task",
			Status:     core.StatusRunning,
		}
		mockRepo.On("GetState", ctx, parentID).Return(parentState, nil)

		// All children successful
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusSuccess},
		}
		mockRepo.On("ListChildren", ctx, parentID).Return(children, nil)

		// Expect parent status to be updated to success
		mockRepo.On("UpsertState", ctx, mock.MatchedBy(func(s *task.State) bool {
			return s.TaskExecID == parentID && s.Status == core.StatusSuccess
		})).Return(nil)

		// Act
		err := manager.UpdateParentStatus(ctx, parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle fail-fast strategy", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockRepo)
		ctx := context.Background()
		parentID := core.MustNewID()

		parentState := &task.State{
			TaskExecID: parentID,
			TaskID:     "parent-task",
			Status:     core.StatusRunning,
		}
		mockRepo.On("GetState", ctx, parentID).Return(parentState, nil)

		// One child failed
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusFailed},
			{Status: core.StatusRunning},
		}
		mockRepo.On("ListChildren", ctx, parentID).Return(children, nil)

		// Expect parent to fail immediately
		mockRepo.On("UpsertState", ctx, mock.MatchedBy(func(s *task.State) bool {
			return s.TaskExecID == parentID && s.Status == core.StatusFailed
		})).Return(nil)

		// Act
		err := manager.UpdateParentStatus(ctx, parentID, task.StrategyFailFast)

		// Assert
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should skip update when no children exist", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockRepo)
		ctx := context.Background()
		parentID := core.MustNewID()

		parentState := &task.State{
			TaskExecID: parentID,
			TaskID:     "parent-task",
			Status:     core.StatusRunning,
		}
		mockRepo.On("GetState", ctx, parentID).Return(parentState, nil)

		// No children
		mockRepo.On("ListChildren", ctx, parentID).Return([]*task.State{}, nil)

		// Should not call UpsertState
		mockRepo.AssertNotCalled(t, "UpsertState")

		// Act
		err := manager.UpdateParentStatus(ctx, parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle repository errors", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockRepo)
		ctx := context.Background()
		parentID := core.MustNewID()

		// GetState fails
		mockRepo.On("GetState", ctx, parentID).Return(nil, fmt.Errorf("database error"))

		// Act
		err := manager.UpdateParentStatus(ctx, parentID, task.StrategyWaitAll)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to batch get parent states")
		assert.Contains(t, err.Error(), "database error")
		mockRepo.AssertExpectations(t)
	})
}

func TestDefaultParentStatusManager_GetAggregatedStatus(t *testing.T) {
	t.Run("Should return success when no children exist", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockRepo)
		ctx := context.Background()
		parentID := core.MustNewID()

		mockRepo.On("ListChildren", ctx, parentID).Return([]*task.State{}, nil)

		// Act
		status, err := manager.GetAggregatedStatus(ctx, parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should aggregate status with wait-all strategy", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockRepo)
		ctx := context.Background()
		parentID := core.MustNewID()

		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusRunning},
		}
		mockRepo.On("ListChildren", ctx, parentID).Return(children, nil)

		// Act
		status, err := manager.GetAggregatedStatus(ctx, parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusRunning, status)
		mockRepo.AssertExpectations(t)
	})
}

func TestDefaultParentStatusManager_StrategyCalculations(t *testing.T) {
	mockRepo := &store.MockTaskRepo{}
	manager := NewParentStatusManager(mockRepo).(*DefaultParentStatusManager)

	t.Run("Should calculate wait-all status correctly", func(t *testing.T) {
		testCases := []struct {
			name           string
			children       []*task.State
			expectedStatus core.StatusType
		}{
			{
				name: "all success",
				children: []*task.State{
					{Status: core.StatusSuccess},
					{Status: core.StatusSuccess},
				},
				expectedStatus: core.StatusSuccess,
			},
			{
				name: "one failed",
				children: []*task.State{
					{Status: core.StatusSuccess},
					{Status: core.StatusFailed},
				},
				expectedStatus: core.StatusFailed,
			},
			{
				name: "one running",
				children: []*task.State{
					{Status: core.StatusSuccess},
					{Status: core.StatusRunning},
				},
				expectedStatus: core.StatusRunning,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				status := manager.calculateWaitAllStatus(tc.children)
				assert.Equal(t, tc.expectedStatus, status)
			})
		}
	})

	t.Run("Should calculate fail-fast status correctly", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusFailed},
			{Status: core.StatusRunning},
		}

		// Act
		status := manager.calculateFailFastStatus(children)

		// Assert
		assert.Equal(t, core.StatusFailed, status)
	})

	t.Run("Should calculate best-effort status correctly", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusFailed},
			{Status: core.StatusSuccess},
			{Status: core.StatusFailed},
		}

		// Act
		status := manager.calculateBestEffortStatus(children)

		// Assert
		assert.Equal(t, core.StatusSuccess, status)
	})

	t.Run("Should calculate race status correctly", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusFailed},
			{Status: core.StatusRunning},
			{Status: core.StatusSuccess},
		}

		// Act
		status := manager.calculateRaceStatus(children)

		// Assert
		assert.Equal(t, core.StatusSuccess, status)
	})

	t.Run("Should use wait-all for empty strategy", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusRunning},
		}

		// Act
		status := manager.calculateStatusWithStrategy(children, "")

		// Assert
		assert.Equal(t, core.StatusRunning, status)
	})

	t.Run("Should default to wait-all for unknown strategy", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusRunning},
		}

		// Act
		status := manager.calculateStatusWithStrategy(children, task.ParallelStrategy("unknown"))

		// Assert
		assert.Equal(t, core.StatusRunning, status)
	})
}

func TestDefaultParentStatusManager_BatchProcessing(t *testing.T) {
	t.Run("Should process single batch when updates are less than batch size", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManagerWithConfig(mockRepo, 10, true) // batch size 10
		ctx := context.Background()

		// Create 5 updates (less than batch size)
		updates := []ParentUpdate{
			{ParentID: core.MustNewID(), Strategy: task.StrategyWaitAll},
			{ParentID: core.MustNewID(), Strategy: task.StrategyFailFast},
			{ParentID: core.MustNewID(), Strategy: task.StrategyBestEffort},
			{ParentID: core.MustNewID(), Strategy: task.StrategyRace},
			{ParentID: core.MustNewID(), Strategy: task.StrategyWaitAll},
		}

		// Mock parent states
		parentStates := make([]*task.State, len(updates))
		for i, update := range updates {
			parentState := &task.State{
				TaskExecID: update.ParentID,
				TaskID:     "parent-task-" + strconv.Itoa(i),
				Status:     core.StatusRunning,
			}
			parentStates[i] = parentState
			mockRepo.On("GetState", ctx, update.ParentID).Return(parentState, nil)

			// Mock children for each parent
			children := []*task.State{
				{Status: core.StatusSuccess},
				{Status: core.StatusSuccess},
			}
			mockRepo.On("ListChildren", ctx, update.ParentID).Return(children, nil)

			// Mock update
			mockRepo.On("UpsertState", ctx, mock.MatchedBy(func(s *task.State) bool {
				return s.TaskExecID == update.ParentID && s.Status == core.StatusSuccess
			})).Return(nil)
		}

		// Act
		err := manager.(*DefaultParentStatusManager).updateParentStatusBatch(ctx, updates)

		// Assert
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should process multiple batches when updates exceed batch size", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManagerWithConfig(mockRepo, 3, true) // batch size 3
		ctx := context.Background()

		// Create 7 updates (will require 3 batches: 3, 3, 1)
		updates := make([]ParentUpdate, 7)
		for i := 0; i < 7; i++ {
			updates[i] = ParentUpdate{
				ParentID: core.MustNewID(),
				Strategy: task.StrategyWaitAll,
			}

			// Mock for each parent
			parentState := &task.State{
				TaskExecID: updates[i].ParentID,
				TaskID:     "parent-task-" + strconv.Itoa(i),
				Status:     core.StatusRunning,
			}
			mockRepo.On("GetState", ctx, updates[i].ParentID).Return(parentState, nil)

			children := []*task.State{{Status: core.StatusSuccess}}
			mockRepo.On("ListChildren", ctx, updates[i].ParentID).Return(children, nil)
			mockRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil)
		}

		// Act
		err := manager.(*DefaultParentStatusManager).updateParentStatusBatch(ctx, updates)

		// Assert
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
		// Verify that GetState was called exactly 7 times
		mockRepo.AssertNumberOfCalls(t, "GetState", 7)
	})

	t.Run("Should handle duplicate parent IDs efficiently", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManagerWithConfig(mockRepo, 10, true)
		ctx := context.Background()

		parentID := core.MustNewID()
		// Create updates with duplicate parent IDs
		updates := []ParentUpdate{
			{ParentID: parentID, Strategy: task.StrategyWaitAll},
			{ParentID: parentID, Strategy: task.StrategyFailFast}, // duplicate
			{ParentID: core.MustNewID(), Strategy: task.StrategyBestEffort},
		}

		// Mock for unique parent IDs
		parentState1 := &task.State{
			TaskExecID: parentID,
			TaskID:     "parent-task-1",
			Status:     core.StatusRunning,
		}
		parentState2 := &task.State{
			TaskExecID: updates[2].ParentID,
			TaskID:     "parent-task-2",
			Status:     core.StatusRunning,
		}

		// After fix, should only call GetState twice (not three times)
		mockRepo.On("GetState", ctx, parentID).Return(parentState1, nil).Once()
		mockRepo.On("GetState", ctx, updates[2].ParentID).Return(parentState2, nil).Once()

		children := []*task.State{{Status: core.StatusSuccess}}
		mockRepo.On("ListChildren", ctx, parentID).Return(children, nil).Once()
		mockRepo.On("ListChildren", ctx, updates[2].ParentID).Return(children, nil).Once()

		mockRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil)

		// Act
		err := manager.(*DefaultParentStatusManager).updateParentStatusBatch(ctx, updates)

		// Assert
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should work with non-batching repository", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManagerWithConfig(mockRepo, 2, true)
		ctx := context.Background()

		// Create 3 updates
		updates := make([]ParentUpdate, 3)
		for i := 0; i < 3; i++ {
			updates[i] = ParentUpdate{
				ParentID: core.MustNewID(),
				Strategy: task.StrategyWaitAll,
			}

			parentState := &task.State{
				TaskExecID: updates[i].ParentID,
				TaskID:     "parent-task-" + strconv.Itoa(i),
				Status:     core.StatusRunning,
			}
			mockRepo.On("GetState", ctx, updates[i].ParentID).Return(parentState, nil)

			children := []*task.State{{Status: core.StatusSuccess}}
			mockRepo.On("ListChildren", ctx, updates[i].ParentID).Return(children, nil)
			mockRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil)
		}

		// Act - should work even without batch repository interface
		err := manager.(*DefaultParentStatusManager).updateParentStatusBatch(ctx, updates)

		// Assert
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle errors in batch processing", func(t *testing.T) {
		// Arrange
		mockRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManagerWithConfig(mockRepo, 2, true)
		ctx := context.Background()

		updates := []ParentUpdate{
			{ParentID: core.MustNewID(), Strategy: task.StrategyWaitAll},
			{ParentID: core.MustNewID(), Strategy: task.StrategyWaitAll},
			{ParentID: core.MustNewID(), Strategy: task.StrategyWaitAll},
		}

		// First batch succeeds
		for i := 0; i < 2; i++ {
			parentState := &task.State{
				TaskExecID: updates[i].ParentID,
				TaskID:     "parent-task-" + strconv.Itoa(i),
				Status:     core.StatusRunning,
			}
			mockRepo.On("GetState", ctx, updates[i].ParentID).Return(parentState, nil)
			children := []*task.State{{Status: core.StatusSuccess}}
			mockRepo.On("ListChildren", ctx, updates[i].ParentID).Return(children, nil)
			mockRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil)
		}

		// Second batch fails
		mockRepo.On("GetState", ctx, updates[2].ParentID).Return(nil, fmt.Errorf("database error"))

		// Act
		err := manager.(*DefaultParentStatusManager).updateParentStatusBatch(ctx, updates)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to process batch 2-3")
		assert.Contains(t, err.Error(), "database error")
		mockRepo.AssertExpectations(t)
	})
}
