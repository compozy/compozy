package activities

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/sethvargo/go-retry"
)

const ExecuteSubtaskLabel = "ExecuteSubtask"

type ExecuteSubtaskInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	ParentStateID  core.ID `json:"parent_state_id"` // was *task.State
	TaskExecID     string  `json:"task_exec_id"`
}

type ExecuteSubtask struct {
	loadWorkflowUC *uc.LoadWorkflow
	executeTaskUC  *uc.ExecuteTask
	taskResponder  *services.TaskResponder
	taskRepo       task.Repository
	configStore    services.ConfigStore
}

// NewExecuteSubtask creates a new ExecuteSubtask activity
func NewExecuteSubtask(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime *runtime.Manager,
	configStore services.ConfigStore,
) *ExecuteSubtask {
	return &ExecuteSubtask{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		executeTaskUC:  uc.NewExecuteTask(runtime),
		taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
		taskRepo:       taskRepo,
		configStore:    configStore,
	}
}

func (a *ExecuteSubtask) Run(ctx context.Context, input *ExecuteSubtaskInput) (*task.SubtaskResponse, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, err
	}

	// Load task config from store
	taskConfig, err := a.configStore.Get(ctx, input.TaskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to load task config for taskExecID %s: %w", input.TaskExecID, err)
	}

	// Normalize task configuration
	if err := a.normalizeTaskConfig(ctx, workflowState, workflowConfig, taskConfig); err != nil {
		return nil, err
	}

	// Get child state with retry logic
	taskState, err := a.getChildStateWithRetry(ctx, input.ParentStateID, taskConfig.ID)
	if err != nil {
		return nil, err
	}
	output, executionError := a.executeTaskUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig: taskConfig,
	})

	// Update task status and output based on execution result
	if executionError != nil {
		taskState.Status = core.StatusFailed
		taskState.Output = nil // Clear output on failure to prevent partial data
		// Convert error to core.Error if needed
		if coreErr, ok := executionError.(*core.Error); ok {
			taskState.Error = coreErr
		} else {
			taskState.Error = core.NewError(executionError, "EXECUTION_ERROR", nil)
		}
	} else {
		taskState.Status = core.StatusSuccess
		taskState.Output = output // Only set output on success
	}
	// Manual timestamp update: Required because the database schema doesn't use
	// ON UPDATE CURRENT_TIMESTAMP for the updated_at column, and the ORM doesn't
	// automatically manage timestamps. This ensures the change is properly tracked.
	taskState.UpdatedAt = time.Now()
	if err := a.taskRepo.UpsertState(ctx, taskState); err != nil {
		return nil, fmt.Errorf("failed to persist task output and status: %w", err)
	}
	// Handle subtask response
	response, err := a.taskResponder.HandleSubtask(ctx, &services.SubtaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      taskState,
		TaskConfig:     taskConfig,
		ExecutionError: executionError,
	})
	if err != nil {
		return nil, err
	}

	// Return the response with the execution status embedded.
	// We only return an error if there's an infrastructure issue that Temporal should retry.
	// Business logic failures are captured in the response status.
	return response, nil
}

// normalizeTaskConfig normalizes the task configuration to avoid race conditions
func (a *ExecuteSubtask) normalizeTaskConfig(
	ctx context.Context,
	wState *workflow.State,
	wConfig *workflow.Config,
	taskConfig *task.Config,
) error {
	// Create a copy of workflow config to avoid race conditions when multiple goroutines
	// concurrently call NormalizeConfig.Execute which mutates the config in-place
	wcCopy, err := wConfig.Clone()
	if err != nil {
		return fmt.Errorf("failed to clone workflow config: %w", err)
	}
	wcCopy.Tasks = append([]task.Config(nil), wConfig.Tasks...)
	tcCopy, err := taskConfig.Clone()
	if err != nil {
		return fmt.Errorf("failed to clone task config: %w", err)
	}
	normalizer := uc.NewNormalizeConfig()
	normalizeInput := &uc.NormalizeConfigInput{
		WorkflowState:  wState,
		WorkflowConfig: wcCopy,
		TaskConfig:     tcCopy,
	}
	return normalizer.Execute(ctx, normalizeInput)
}

// getChildStateWithRetry retrieves child state with exponential backoff retry
func (a *ExecuteSubtask) getChildStateWithRetry(
	ctx context.Context,
	parentStateID core.ID,
	taskID string,
) (*task.State, error) {
	var taskState *task.State
	err := retry.Do(
		ctx,
		retry.WithMaxRetries(5, retry.NewExponential(50*time.Millisecond)),
		func(ctx context.Context) error {
			var err error
			taskState, err = a.getChildState(ctx, parentStateID, taskID)
			if err != nil {
				// If the error is anything other than Not Found, fail immediately (non-retryable)
				if !errors.Is(err, store.ErrTaskNotFound) {
					return fmt.Errorf("failed to get child state: %w", err)
				}
				// ErrTaskNotFound is retryable
				return retry.RetryableError(err)
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("child state for task %s not found after retries: %w", taskID, err)
	}
	// Add explicit nil check in case repository returns (nil, nil)
	if taskState == nil {
		return nil, fmt.Errorf("child state for task %s returned nil without error", taskID)
	}
	return taskState, nil
}

// getChildState retrieves the existing child state for a specific task
func (a *ExecuteSubtask) getChildState(
	ctx context.Context,
	parentStateID core.ID,
	taskID string,
) (*task.State, error) {
	// Use optimized direct lookup instead of fetching all children
	return a.taskRepo.GetChildByTaskID(ctx, parentStateID, taskID)
}
