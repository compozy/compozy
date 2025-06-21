package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func TestWaitTaskManager_UpdateWaitTaskStatus(t *testing.T) {
	t.Run("Should update wait task status to success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		output := &core.Output{
			"signal_received": true,
			"condition_met":   true,
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		taskRepo.On("UpsertState", ctx, mock.MatchedBy(func(state *task.State) bool {
			return state.Status == core.StatusSuccess &&
				state.Output != nil
		})).Return(nil)
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.UpdateWaitTaskStatus(ctx, taskExecID, core.StatusSuccess, output)
		// Assert
		assert.NoError(t, err)
		taskRepo.AssertExpectations(t)
	})
	t.Run("Should update parent state when task is a child", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		parentStateID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
			ParentStateID:  &parentStateID,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		parentState := &task.State{
			TaskID:         "parent-task",
			TaskExecID:     parentStateID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: taskState.WorkflowExecID,
			Status:         core.StatusRunning,
			ExecutionType:  task.ExecutionParallel,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		taskRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil)
		// Mock transaction methods
		taskRepo.On("GetStateForUpdate", ctx, mock.Anything, parentStateID).Return(parentState, nil)
		taskRepo.On("GetProgressInfo", ctx, parentStateID).Return(&task.ProgressInfo{
			TotalChildren:  1,
			CompletedCount: 1,
			FailedCount:    0,
			RunningCount:   0,
			PendingCount:   0,
			CompletionRate: 100.0,
			FailureRate:    0.0,
		}, nil)
		taskRepo.On("UpsertStateWithTx", ctx, mock.Anything, mock.AnythingOfType("*task.State")).Return(nil)
		taskRepo.On("WithTx", ctx, mock.AnythingOfType("func(pgx.Tx) error")).
			Return(nil).
			Run(func(args mock.Arguments) {
				// Simulate transaction execution
				fn := args.Get(1).(func(pgx.Tx) error)
				// We don't actually have a transaction, but the mock methods will be called
				fn(nil)
			})
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.UpdateWaitTaskStatus(ctx, taskExecID, core.StatusSuccess, nil)
		// Assert
		assert.NoError(t, err)
		taskRepo.AssertExpectations(t)
	})
	t.Run("Should handle task state retrieval error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(nil, errors.New("database error"))
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.UpdateWaitTaskStatus(ctx, taskExecID, core.StatusSuccess, nil)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get task state")
		taskRepo.AssertExpectations(t)
	})
	t.Run("Should handle upsert state error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		taskRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(errors.New("database error"))
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.UpdateWaitTaskStatus(ctx, taskExecID, core.StatusSuccess, nil)
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update task state")
		taskRepo.AssertExpectations(t)
	})
}

func TestWaitTaskManager_ValidateWaitTaskSignal(t *testing.T) {
	t.Run("Should validate signal successfully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.ValidateWaitTaskSignal(ctx, taskExecID, "approval_signal")
		// Assert
		assert.NoError(t, err)
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
	})
	t.Run("Should reject signal for completed task", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusSuccess,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.ValidateWaitTaskSignal(ctx, taskExecID, "approval_signal")
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task is not waiting for signals")
		taskRepo.AssertExpectations(t)
	})
	t.Run("Should reject wrong signal name", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.ValidateWaitTaskSignal(ctx, taskExecID, "wrong_signal")
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task is waiting for signal 'approval_signal', not 'wrong_signal'")
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
	})
	t.Run("Should reject non-wait task", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "basic-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusRunning,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "basic-task",
				Type: task.TaskTypeBasic,
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		err := manager.ValidateWaitTaskSignal(ctx, taskExecID, "any_signal")
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task is not a wait task")
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
	})
}

func TestWaitTaskManager_PrepareWaitTaskResponse(t *testing.T) {
	t.Run("Should prepare response successfully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		workflowExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusSuccess,
			Output: &core.Output{
				"result": "completed",
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		workflowState := &workflow.State{
			WorkflowID:     "test-workflow",
			WorkflowExecID: workflowExecID,
			Status:         core.StatusRunning,
		}
		// Set up mocks
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		workflowRepo.On("GetState", ctx, workflowExecID).Return(workflowState, nil)
		taskRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil)
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		response, err := manager.PrepareWaitTaskResponse(ctx, taskState, workflowConfig)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, taskState, response.State)
		assert.Equal(t, core.StatusSuccess, response.State.Status)
		configStore.AssertExpectations(t)
		workflowRepo.AssertExpectations(t)
	})
	t.Run("Should handle config load error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(MockConfigStore)
		workflowRepo := new(store.MockWorkflowRepo)
		taskResponder := NewTaskResponder(workflowRepo, taskRepo)
		parentUpdater := NewParentStatusUpdater(taskRepo)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusSuccess,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		workflowConfig := &workflow.Config{
			ID: "test-workflow",
		}
		// Set up mocks
		configStore.On("Get", ctx, taskExecID.String()).Return(nil, errors.New("config not found"))
		// Create manager
		manager := NewWaitTaskManager(taskRepo, configStore, taskResponder, parentUpdater)
		// Act
		response, err := manager.PrepareWaitTaskResponse(ctx, taskState, workflowConfig)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to load task config")
		configStore.AssertExpectations(t)
	})
}
