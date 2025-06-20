package uc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
)

type MockConditionEvaluator struct {
	mock.Mock
}

func (m *MockConditionEvaluator) Evaluate(
	ctx context.Context,
	condition string,
	evalContext map[string]any,
) (bool, error) {
	args := m.Called(ctx, condition, evalContext)
	return args.Bool(0), args.Error(1)
}

func TestProcessWaitSignal_Execute(t *testing.T) {
	t.Run("Should process signal successfully when condition is met", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
			Input:          &core.Input{"key": "value"},
			Output: &core.Output{
				"processor_output": map[string]any{
					"result": "processed",
				},
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `signal.payload.approved == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "approval_signal",
			Payload: map[string]any{
				"approved": true,
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		evaluator.On("Evaluate", ctx, taskConfig.Condition, mock.AnythingOfType("map[string]interface {}")).
			Return(true, nil)
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.True(t, output.ConditionMet)
		assert.Empty(t, output.Error)
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
		evaluator.AssertExpectations(t)
	})
	t.Run("Should return false when signal name doesn't match", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `signal.payload.approved == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "wrong_signal",
			Payload: map[string]any{
				"approved": true,
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.False(t, output.ConditionMet)
		assert.Empty(t, output.Error)
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
		evaluator.AssertExpectations(t)
	})
	t.Run("Should handle task not found error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "test_signal",
			Payload:    map[string]any{},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(nil, errors.New("task not found"))
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "failed to get task state")
		// Verify mocks
		taskRepo.AssertExpectations(t)
	})
	t.Run("Should handle config retrieval error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "test_signal",
			Payload:    map[string]any{},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(nil, errors.New("config not found"))
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "failed to load task config")
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
	})
	t.Run("Should reject non-wait task", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		taskState := &task.State{
			TaskID:         "basic-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "basic-task",
				Type: task.TaskTypeBasic, // Not a wait task
			},
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "test_signal",
			Payload:    map[string]any{},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "is not a wait task")
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
	})
	t.Run("Should reject task not in waiting state", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusSuccess, // Not in waiting state
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeWait,
			},
			WaitTask: task.WaitTask{
				WaitFor: "test_signal",
			},
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "test_signal",
			Payload:    map[string]any{},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "is not in waiting state")
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
	})
	t.Run("Should handle condition evaluation error", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `signal.payload.approved == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "approval_signal",
			Payload: map[string]any{
				"approved": true,
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		evaluator.On("Evaluate", ctx, taskConfig.Condition, mock.AnythingOfType("map[string]interface {}")).
			Return(false, errors.New("CEL evaluation error"))
		// Expect task state to be updated to FAILED
		taskRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil)
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "condition evaluation failed")
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
		evaluator.AssertExpectations(t)
	})
	t.Run("Should return false when condition is not met", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `signal.payload.approved == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
			},
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "approval_signal",
			Payload: map[string]any{
				"approved": false,
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		evaluator.On("Evaluate", ctx, taskConfig.Condition, mock.AnythingOfType("map[string]interface {}")).
			Return(false, nil)
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.False(t, output.ConditionMet)
		assert.Empty(t, output.Error)
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
		evaluator.AssertExpectations(t)
	})
	t.Run("Should include processor output when available", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		taskExecID := core.MustNewID()
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		evaluator := new(MockConditionEvaluator)
		processorOutput := &core.Output{
			"validated": true,
			"score":     95,
		}
		taskState := &task.State{
			TaskID:         "wait-task",
			TaskExecID:     taskExecID,
			WorkflowID:     "test-workflow",
			WorkflowExecID: core.MustNewID(),
			Status:         core.StatusWaiting,
			Output: &core.Output{
				"processor_output": processorOutput,
			},
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `signal.payload.approved == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "approval_signal",
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "validator",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		input := &ProcessWaitSignalInput{
			TaskExecID: taskExecID,
			SignalName: "approval_signal",
			Payload: map[string]any{
				"approved": true,
			},
		}
		// Set up mocks
		taskRepo.On("GetState", ctx, taskExecID).Return(taskState, nil)
		configStore.On("Get", ctx, taskExecID.String()).Return(taskConfig, nil)
		evaluator.On("Evaluate", ctx, taskConfig.Condition, mock.AnythingOfType("map[string]interface {}")).
			Return(true, nil)
		// Create use case
		uc := NewProcessWaitSignal(taskRepo, configStore, evaluator)
		// Act
		output, err := uc.Execute(ctx, input)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.True(t, output.ConditionMet)
		assert.NotNil(t, output.ProcessorOutput)
		assert.Equal(t, processorOutput, output.ProcessorOutput)
		// Verify mocks
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
		evaluator.AssertExpectations(t)
	})
}
