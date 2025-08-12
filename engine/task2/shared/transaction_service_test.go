package shared

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
)

func TestTransactionService_SaveStateWithLocking(t *testing.T) {
	t.Run("Should return error when state is nil", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		service := NewTransactionService(mockTaskRepo)
		ctx := context.Background()

		// Act
		err := service.SaveStateWithLocking(ctx, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "task state cannot be nil")
		mockTaskRepo.AssertNotCalled(t, "UpsertState")
	})

	t.Run("Should save state with transaction when repo supports transactions", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		service := NewTransactionService(mockTaskRepo)
		ctx := context.Background()

		state := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "test-task",
			Status:     core.StatusRunning,
		}

		// Mock transaction behavior
		mockTaskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).
			Return(nil).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(pgx.Tx) error)
				// Execute the function to trigger internal mocks
				fn(nil)
			})
		mockTaskRepo.On("GetStateForUpdate", ctx, mock.Anything, state.TaskExecID).Return(state, nil)
		mockTaskRepo.On("UpsertStateWithTx", ctx, mock.Anything, state).Return(nil)

		// Act
		err := service.SaveStateWithLocking(ctx, state)

		// Assert
		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should handle error from transaction", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		service := NewTransactionService(mockTaskRepo)
		ctx := context.Background()

		state := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "test-task",
			Status:     core.StatusRunning,
		}

		saveError := errors.New("database error")
		// Mock transaction that returns error
		mockTaskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).Return(saveError)

		// Act
		err := service.SaveStateWithLocking(ctx, state)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		mockTaskRepo.AssertExpectations(t)
	})
}

func TestTransactionService_ApplyTransformation(t *testing.T) {
	t.Run("Should apply transformation with transaction support", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		service := NewTransactionService(mockTaskRepo)
		ctx := context.Background()

		taskExecID := core.MustNewID()
		state := &task.State{
			TaskExecID: taskExecID,
			TaskID:     "test-task",
			Status:     core.StatusRunning,
		}

		// Mock transaction behavior
		mockTaskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).
			Return(nil).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(pgx.Tx) error)
				// Execute the function to trigger internal mocks
				fn(nil)
			})
		mockTaskRepo.On("GetStateForUpdate", ctx, mock.Anything, taskExecID).Return(state, nil)
		mockTaskRepo.On("UpsertStateWithTx", ctx, mock.Anything, mock.MatchedBy(func(s *task.State) bool {
			return s.Status == core.StatusSuccess
		})).Return(nil)

		transformer := func(s *task.State) error {
			s.Status = core.StatusSuccess
			return nil
		}

		// Act
		err := service.ApplyTransformation(ctx, taskExecID, transformer)

		// Assert
		require.NoError(t, err)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should handle transformation error", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		service := NewTransactionService(mockTaskRepo)
		ctx := context.Background()

		taskExecID := core.MustNewID()
		state := &task.State{
			TaskExecID: taskExecID,
			TaskID:     "test-task",
			Status:     core.StatusRunning,
		}

		transformError := errors.New("transformation failed")
		expectedError := fmt.Errorf("task processing failed: %w", transformError)

		// Mock WithTx to execute the function and return the error it produces
		mockTaskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).
			Return(expectedError).
			Run(func(args mock.Arguments) {
				fn := args.Get(1).(func(pgx.Tx) error)
				// Execute the function to trigger the mocks
				err := fn(nil)
				// Verify the error matches what we expect
				assert.Equal(t, expectedError.Error(), err.Error())
			})

		mockTaskRepo.On("GetStateForUpdate", ctx, mock.Anything, taskExecID).Return(state, nil)

		transformer := func(_ *task.State) error {
			return transformError
		}

		// Act
		err := service.ApplyTransformation(ctx, taskExecID, transformer)

		// Assert
		require.Error(t, err)
		assert.Equal(t, expectedError.Error(), err.Error())
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Should handle transaction error", func(t *testing.T) {
		// Arrange
		mockTaskRepo := &store.MockTaskRepo{}
		service := NewTransactionService(mockTaskRepo)
		ctx := context.Background()

		taskExecID := core.MustNewID()

		// Mock WithTx to return error
		transactionError := errors.New("database connection lost")
		mockTaskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).Return(transactionError)

		transformer := func(s *task.State) error {
			s.Status = core.StatusSuccess
			return nil
		}

		// Act
		err := service.ApplyTransformation(ctx, taskExecID, transformer)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "database connection lost")
		mockTaskRepo.AssertExpectations(t)
	})
}

func TestTransactionService_mergeStateChanges(t *testing.T) {
	t.Run("Should merge only execution-related fields", func(t *testing.T) {
		// Arrange
		service := &TransactionService{}

		target := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "target-task",
			Status:     core.StatusPending,
			Input:      &core.Input{"key": "target-value"},
			Output:     nil,
			Error:      nil,
		}

		source := &task.State{
			TaskExecID: core.MustNewID(), // Different ID - should not be copied
			TaskID:     "source-task",    // Different ID - should not be copied
			Status:     core.StatusSuccess,
			Input:      &core.Input{"key": "source-value"}, // Should not be copied
			Output:     &core.Output{"result": "success"},
			Error:      &core.Error{Message: "test error"},
		}

		originalTargetID := target.TaskExecID
		originalTaskID := target.TaskID
		originalInput := target.Input

		// Act
		service.mergeStateChanges(target, source)

		// Assert
		// Only Status, Output, and Error should be merged
		assert.Equal(t, source.Status, target.Status)
		assert.Equal(t, source.Output, target.Output)
		assert.Equal(t, source.Error, target.Error)

		// Other fields should remain unchanged
		assert.Equal(t, originalTargetID, target.TaskExecID)
		assert.Equal(t, originalTaskID, target.TaskID)
		assert.Equal(t, originalInput, target.Input)
	})

	t.Run("Should backfill Input when missing", func(t *testing.T) {
		// Arrange
		service := &TransactionService{}

		target := &task.State{
			TaskExecID: core.MustNewID(),
			TaskID:     "target-task",
			Status:     core.StatusPending,
			Input:      nil,
		}
		srcInput := &core.Input{"key": "new-value"}
		source := &task.State{
			Status: core.StatusSuccess,
			Input:  srcInput,
		}

		// Act
		service.mergeStateChanges(target, source)

		// Assert
		assert.Equal(t, core.StatusSuccess, target.Status)
		assert.Equal(t, srcInput, target.Input)
	})
}
