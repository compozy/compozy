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
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
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
	task2Factory   task2.Factory
	templateEngine *tplengine.TemplateEngine
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
	task2Factory task2.Factory,
	templateEngine *tplengine.TemplateEngine,
) *ExecuteSubtask {
	return &ExecuteSubtask{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		executeTaskUC:  uc.NewExecuteTask(runtime, nil, nil), // Subtasks don't need memory manager
		task2Factory:   task2Factory,
		templateEngine: templateEngine,
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
	// Use task2 normalizer for subtask
	normalizer, err := a.task2Factory.CreateNormalizer(taskConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to create subtask normalizer: %w", err)
	}
	// Create context builder to build proper normalization context
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	// Build proper normalization context with all template variables
	normContext := contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	// Normalize the task configuration
	if err := normalizer.Normalize(taskConfig, normContext); err != nil {
		return nil, fmt.Errorf("failed to normalize subtask: %w", err)
	}
	// Get child state with retry logic
	taskState, err := a.getChildStateWithRetry(ctx, input.ParentStateID, taskConfig.ID)
	if err != nil {
		return nil, err
	}
	output, executionError := a.executeTaskUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig:     taskConfig,
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		ProjectConfig:  nil, // Subtasks don't use memory operations
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
	// Use task2 ResponseHandler for subtask
	handler, err := a.task2Factory.CreateResponseHandler(taskConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to create subtask response handler: %w", err)
	}
	// Prepare input for response handler
	responseInput := &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
	// Handle the response
	result, err := handler.HandleResponse(ctx, responseInput)
	if err != nil {
		return nil, fmt.Errorf("failed to handle subtask response: %w", err)
	}
	// Convert shared.ResponseOutput to task.SubtaskResponse
	converter := NewResponseConverter()
	// Convert to MainTaskResponse first then extract subtask data
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	subtaskResponse := &task.SubtaskResponse{
		State: mainTaskResponse.State,
	}
	// Return the response with the execution status embedded.
	// We only return an error if there's an infrastructure issue that Temporal should retry.
	// Business logic failures are captured in the response status.
	return subtaskResponse, nil
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
