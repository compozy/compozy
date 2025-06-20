package activities

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
	"github.com/compozy/compozy/engine/workflow"
)

func TestExecuteWait_Run(t *testing.T) {
	t.Run("Should create wait task state successfully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()
		workflowRepo := new(store.MockWorkflowRepo)
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		cwd, _ := core.CWDFromPath("/test")
		workflowConfig := &workflow.Config{
			ID: workflowID,
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:        "wait-task",
						Type:      task.TaskTypeWait,
						Condition: `signal.payload.approved == true`,
					},
					WaitTask: task.WaitTask{
						WaitFor: "approval_signal",
					},
				},
			},
		}
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
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
		// Set up mocks
		workflowRepo.On("GetState", ctx, workflowExecID).Return(workflowState, nil)
		configStore.On("Save", ctx, mock.AnythingOfType("string"), taskConfig).Return(nil)
		taskRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil).Run(func(args mock.Arguments) {
			state := args.Get(1).(*task.State)
			state.TaskExecID = taskExecID
		})
		// Create activity
		activity := NewExecuteWait(
			[]*workflow.Config{workflowConfig},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
		)
		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}
		// Act
		response, err := activity.Run(ctx, input)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "wait-task", response.State.TaskID)
		// Verify task state was created with proper metadata
		taskRepo.AssertCalled(t, "UpsertState", ctx, mock.MatchedBy(func(state *task.State) bool {
			return state.TaskID == "wait-task" &&
				state.ExecutionType == task.ExecutionWait &&
				state.Output != nil &&
				(*state.Output)["wait_status"] == "waiting" &&
				(*state.Output)["signal_name"] == "approval_signal" &&
				(*state.Output)["has_processor"] == false
		}))
		workflowRepo.AssertExpectations(t)
		taskRepo.AssertExpectations(t)
		configStore.AssertExpectations(t)
	})
	t.Run("Should handle nil task config", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/tmp")
		workflowRepo := new(store.MockWorkflowRepo)
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		// Create activity
		activity := NewExecuteWait(
			[]*workflow.Config{},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
		)
		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     nil, // nil config
		}
		// Act
		response, err := activity.Run(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "task_config is required")
	})
	t.Run("Should handle workflow not found", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/tmp")
		workflowRepo := new(store.MockWorkflowRepo)
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
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
		workflowRepo.On("GetState", ctx, workflowExecID).Return(nil, errors.New("workflow not found"))
		// Create activity
		activity := NewExecuteWait(
			[]*workflow.Config{},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
		)
		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}
		// Act
		response, err := activity.Run(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "workflow not found")
		// Verify mocks
		workflowRepo.AssertExpectations(t)
	})
	t.Run("Should handle wrong task type", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		cwd, _ := core.CWDFromPath("/tmp")
		workflowRepo := new(store.MockWorkflowRepo)
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		workflowConfig := &workflow.Config{
			ID: workflowID,
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:   "wait-task",
						Type: task.TaskTypeWait,
					},
				},
			},
		}
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:   "wait-task",
				Type: task.TaskTypeBasic, // Wrong type
			},
		}
		// Set up mocks
		workflowRepo.On("GetState", ctx, workflowExecID).Return(workflowState, nil)
		// Create activity
		activity := NewExecuteWait(
			[]*workflow.Config{workflowConfig},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
		)
		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}
		// Act
		response, err := activity.Run(ctx, input)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "unsupported task type")
		// Verify mocks
		workflowRepo.AssertExpectations(t)
	})
	t.Run("Should include processor metadata when processor is configured", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		workflowID := "test-workflow"
		workflowExecID := core.MustNewID()
		taskExecID := core.MustNewID()
		workflowRepo := new(store.MockWorkflowRepo)
		taskRepo := new(store.MockTaskRepo)
		configStore := new(services.MockConfigStore)
		cwd, _ := core.CWDFromPath("/test")
		workflowConfig := &workflow.Config{
			ID: workflowID,
			Tasks: []task.Config{
				{
					BaseConfig: task.BaseConfig{
						ID:        "wait-task",
						Type:      task.TaskTypeWait,
						Condition: `processor.output.valid == true`,
					},
					WaitTask: task.WaitTask{
						WaitFor: "data_signal",
						Processor: &task.Config{
							BaseConfig: task.BaseConfig{
								ID:   "validator",
								Type: task.TaskTypeBasic,
							},
						},
					},
				},
			},
		}
		workflowState := &workflow.State{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			Status:         core.StatusPending,
		}
		taskConfig := &task.Config{
			BaseConfig: task.BaseConfig{
				ID:        "wait-task",
				Type:      task.TaskTypeWait,
				Condition: `processor.output.valid == true`,
			},
			WaitTask: task.WaitTask{
				WaitFor: "data_signal",
				Processor: &task.Config{
					BaseConfig: task.BaseConfig{
						ID:   "validator",
						Type: task.TaskTypeBasic,
					},
				},
			},
		}
		// Set up mocks
		workflowRepo.On("GetState", ctx, workflowExecID).Return(workflowState, nil)
		configStore.On("Save", ctx, mock.AnythingOfType("string"), taskConfig).Return(nil)
		taskRepo.On("UpsertState", ctx, mock.AnythingOfType("*task.State")).Return(nil).Run(func(args mock.Arguments) {
			state := args.Get(1).(*task.State)
			state.TaskExecID = taskExecID
		})
		// Create activity
		activity := NewExecuteWait(
			[]*workflow.Config{workflowConfig},
			workflowRepo,
			taskRepo,
			configStore,
			cwd,
		)
		input := &ExecuteWaitInput{
			WorkflowID:     workflowID,
			WorkflowExecID: workflowExecID,
			TaskConfig:     taskConfig,
		}
		// Act
		response, err := activity.Run(ctx, input)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, response)
		// Verify has_processor is true
		taskRepo.AssertCalled(t, "UpsertState", ctx, mock.MatchedBy(func(state *task.State) bool {
			return state.Output != nil && (*state.Output)["has_processor"] == true
		}))
	})
}
