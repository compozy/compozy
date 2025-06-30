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
)

func TestNewParentStatusManager(t *testing.T) {
	t.Run("Should create parent status manager with task repository", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}

		// Act
		manager := NewParentStatusManager(mockTaskRepo)

		// Assert
		assert.NotNil(t, manager)
		_, ok := manager.(*DefaultParentStatusManager)
		assert.True(t, ok)
	})
}

func TestDefaultParentStatusManager_UpdateParentStatus(t *testing.T) {
	t.Run("Should update parent status with wait_all strategy", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
		}
		parentState := &task.State{
			TaskExecID: parentID,
			Status:     core.StatusRunning,
		}

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)
		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)
		mockTaskRepo.On("UpsertState", mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.TaskExecID == parentID && state.Status == core.StatusSuccess
		})).Return(nil)

		// Act
		err := manager.UpdateParentStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should not update when parent already has expected status", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
		}
		parentState := &task.State{
			TaskExecID: parentID,
			Status:     core.StatusSuccess, // Already success
		}

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)
		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)
		// UpsertState should NOT be called since status hasn't changed

		// Act
		err := manager.UpdateParentStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should update parent status to failed when child fails", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
			{TaskExecID: core.MustNewID(), Status: core.StatusFailed},
		}
		parentState := &task.State{
			TaskExecID: parentID,
			Status:     core.StatusRunning,
		}

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)
		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)
		mockTaskRepo.On("UpsertState", mock.Anything, mock.MatchedBy(func(state *task.State) bool {
			return state.Status == core.StatusFailed
		})).Return(nil)

		// Act
		err := manager.UpdateParentStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should handle empty children list", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{}
		parentState := &task.State{
			TaskExecID: parentID,
			Status:     core.StatusRunning,
		}

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)
		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)

		// Act
		err := manager.UpdateParentStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should handle error when listing children", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		parentState := &task.State{
			TaskExecID: parentID,
			Status:     core.StatusRunning,
		}
		listError := errors.New("database error")

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return(parentState, nil)
		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return([]*task.State(nil), listError)

		// Act
		err := manager.UpdateParentStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list children")
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should handle error when getting parent state", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		getStateError := errors.New("state not found")

		mockTaskRepo.On("GetState", mock.Anything, parentID).Return((*task.State)(nil), getStateError)

		// Act
		err := manager.UpdateParentStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get parent state")
		mockTaskRepo.AssertExpectations(t)
	})
}

func TestDefaultParentStatusManager_GetAggregatedStatus(t *testing.T) {
	t.Run("Should return success when all children succeed", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
		}

		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)

		// Act
		status, err := manager.GetAggregatedStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, status)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should return running when some children are running", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
			{TaskExecID: core.MustNewID(), Status: core.StatusRunning},
		}

		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)

		// Act
		status, err := manager.GetAggregatedStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusRunning, status)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should return failed when any child fails", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{
			{TaskExecID: core.MustNewID(), Status: core.StatusSuccess},
			{TaskExecID: core.MustNewID(), Status: core.StatusFailed},
		}

		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)

		// Act
		status, err := manager.GetAggregatedStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusFailed, status)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should return success for empty children list", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		manager := NewParentStatusManager(mockTaskRepo).(*DefaultParentStatusManager)

		parentID := core.MustNewID()
		children := []*task.State{}

		mockTaskRepo.On("ListChildren", mock.Anything, parentID).Return(children, nil)

		// Act
		status, err := manager.GetAggregatedStatus(context.Background(), parentID, task.StrategyWaitAll)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, core.StatusSuccess, status)
		mockTaskRepo.AssertExpectations(t)
	})
}

func TestDefaultParentStatusManager_StrategyCalculations(t *testing.T) {
	manager := &DefaultParentStatusManager{}

	t.Run("Should calculate fail-fast strategy correctly", func(t *testing.T) {
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

	t.Run("Should calculate best-effort strategy correctly", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusFailed},
		}

		// Act
		status := manager.calculateBestEffortStatus(children)

		// Assert
		assert.Equal(t, core.StatusSuccess, status)
	})

	t.Run("Should calculate race strategy correctly", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusRunning},
		}

		// Act
		status := manager.calculateRaceStatus(children)

		// Assert
		assert.Equal(t, core.StatusSuccess, status)
	})

	t.Run("Should calculate wait-all strategy correctly", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusRunning},
		}

		// Act
		status := manager.calculateWaitAllStatus(children)

		// Assert
		assert.Equal(t, core.StatusRunning, status)
	})
}

func TestDefaultParentStatusManager_CalculateStatusWithStrategy(t *testing.T) {
	manager := &DefaultParentStatusManager{}

	t.Run("Should use fail-fast strategy", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusFailed},
		}

		// Act
		status := manager.calculateStatusWithStrategy(children, task.StrategyFailFast)

		// Assert
		assert.Equal(t, core.StatusFailed, status)
	})

	t.Run("Should use best-effort strategy", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusFailed},
		}

		// Act
		status := manager.calculateStatusWithStrategy(children, task.StrategyBestEffort)

		// Assert
		assert.Equal(t, core.StatusSuccess, status)
	})

	t.Run("Should use race strategy", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusRunning},
		}

		// Act
		status := manager.calculateStatusWithStrategy(children, task.StrategyRace)

		// Assert
		assert.Equal(t, core.StatusSuccess, status)
	})

	t.Run("Should default to wait-all strategy", func(t *testing.T) {
		// Arrange
		children := []*task.State{
			{Status: core.StatusSuccess},
			{Status: core.StatusRunning},
		}

		// Act
		status := manager.calculateStatusWithStrategy(children, task.StrategyWaitAll)

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
