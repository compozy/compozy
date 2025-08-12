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
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
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
	appConfig      *config.Config
}

// NewExecuteSubtask creates a new ExecuteSubtask activity
func NewExecuteSubtask(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime runtime.Runtime,
	configStore services.ConfigStore,
	task2Factory task2.Factory,
	templateEngine *tplengine.TemplateEngine,
	appConfig *config.Config,
) *ExecuteSubtask {
	return &ExecuteSubtask{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		executeTaskUC: uc.NewExecuteTask(
			runtime,
			workflowRepo,
			nil,
			nil,
			appConfig,
		), // Subtasks don't need memory manager
		task2Factory:   task2Factory,
		templateEngine: templateEngine,
		taskRepo:       taskRepo,
		configStore:    configStore,
		appConfig:      appConfig,
	}
}

func (a *ExecuteSubtask) Run(ctx context.Context, input *ExecuteSubtaskInput) (*task.SubtaskResponse, error) {
	log := logger.FromContext(ctx)
	log.Debug("ExecuteSubtask.Run starting",
		"task_exec_id", input.TaskExecID,
		"parent_state_id", input.ParentStateID)
	// Load workflow and task configs
	_, workflowConfig, taskConfig, err := a.loadConfigs(ctx, input)
	if err != nil {
		return nil, err
	}
	log.Debug("Loaded task config",
		"task_id", taskConfig.ID,
		"task_type", taskConfig.Type)
	// ---------------- SEQUENTIAL EXECUTION FOR SIBLINGS ----------------
	// CRITICAL FIX: Wait for prior siblings BEFORE normalization
	// This ensures sibling task outputs are available in the workflow state
	// when templates are parsed during normalization
	log.Debug("About to wait for prior siblings",
		"parent_state_id", input.ParentStateID,
		"current_task_id", taskConfig.ID)
	if err := a.waitForPriorSiblings(ctx, input.ParentStateID, taskConfig.ID); err != nil {
		return nil, fmt.Errorf("failed waiting for sibling tasks: %w", err)
	}
	log.Debug("Finished waiting for prior siblings")
	// Refresh workflow state after siblings complete to get their outputs
	workflowState, _, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to refresh workflow state: %w", err)
	}
	// -------------------------------------------------------------------
	// Normalize task configuration AFTER siblings complete
	// This ensures template interpolation has access to sibling outputs
	if err := a.normalizeTask(ctx, taskConfig, workflowState, workflowConfig, &input.ParentStateID); err != nil {
		return nil, err
	}
	// Execute the task and handle response
	return a.executeAndHandleResponse(ctx, input, taskConfig, workflowState, workflowConfig)
}

func (a *ExecuteSubtask) loadConfigs(
	ctx context.Context,
	input *ExecuteSubtaskInput,
) (*workflow.State, *workflow.Config, *task.Config, error) {
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	// Load task config from store
	taskConfig, err := a.configStore.Get(ctx, input.TaskExecID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load task config for taskExecID %s: %w", input.TaskExecID, err)
	}
	return workflowState, workflowConfig, taskConfig, nil
}

func (a *ExecuteSubtask) normalizeTask(
	_ context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	parentStateID *core.ID,
) error {
	// Use task2 normalizer for subtask
	normalizer, err := a.task2Factory.CreateNormalizer(taskConfig.Type)
	if err != nil {
		return fmt.Errorf("failed to create subtask normalizer: %w", err)
	}
	// Create context builder to build proper normalization context
	contextBuilder, err := shared.NewContextBuilder()
	if err != nil {
		return fmt.Errorf("failed to create context builder: %w", err)
	}
	// Build proper normalization context with all template variables
	normContext := contextBuilder.BuildContextForTaskInstance(
		workflowState,
		workflowConfig,
		taskConfig,
		parentStateID,
	)
	// Normalize the task configuration
	if err := normalizer.Normalize(taskConfig, normContext); err != nil {
		return fmt.Errorf("failed to normalize subtask: %w", err)
	}
	return nil
}

func (a *ExecuteSubtask) executeAndHandleResponse(
	ctx context.Context,
	input *ExecuteSubtaskInput,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) (*task.SubtaskResponse, error) {
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
	handler, err := a.task2Factory.CreateResponseHandler(ctx, taskConfig.Type)
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

// waitForPriorSiblings blocks until every sibling task that appears *before*
// the current task in the parent's Tasks slice has completed (SUCCESS or FAILED).
func (a *ExecuteSubtask) waitForPriorSiblings(
	ctx context.Context,
	parentStateID core.ID,
	currentTaskID string,
) error {
	log := logger.FromContext(ctx)

	// Attempt to load the parent task config via its TaskExecID (same as state ID).
	parentConfig, err := a.configStore.Get(ctx, parentStateID.String())
	if err != nil {
		// If we cannot load the parent config we fall back to previous behavior.
		log.Warn("could not load parent task config; proceeding without sibling ordering",
			"parent_state_id", parentStateID, "error", err)
		return nil
	}
	if parentConfig == nil || len(parentConfig.Tasks) == 0 {
		log.Debug("No parent config or no sibling tasks found",
			"parent_state_id", parentStateID,
			"parent_config_nil", parentConfig == nil)
		return nil // no siblings to wait for
	}
	log.Debug("Found parent config with tasks",
		"parent_state_id", parentStateID,
		"num_tasks", len(parentConfig.Tasks),
		"parent_type", parentConfig.Type)

	// Build ordered list of earlier sibling IDs.
	priorSiblingIDs := a.findPriorSiblingIDs(parentConfig, currentTaskID, log)
	if len(priorSiblingIDs) == 0 {
		log.Debug("First child task - nothing to wait for",
			"current_task_id", currentTaskID)
		return nil // first child — nothing to wait for
	}
	log.Debug("Found prior siblings to wait for",
		"current_task_id", currentTaskID,
		"prior_siblings", priorSiblingIDs)

	// CRITICAL FIX: Use time.Sleep instead of retry.RetryableError to force Temporal
	// activity yield and database cache refresh between polling attempts.
	// This ensures we can observe the second transaction (TX-B) that commits the actual output.
	const (
		pollInterval = 200 * time.Millisecond
		pollTimeout  = 30 * time.Second
	)

	// Poll each prior sibling until it reaches a terminal state.
	for _, siblingID := range priorSiblingIDs {
		if err := a.waitForSingleSibling(
			ctx, parentStateID, siblingID, currentTaskID, log, pollInterval, pollTimeout,
		); err != nil {
			return err
		}
	}
	return nil
}

// findPriorSiblingIDs returns the IDs of siblings that come before the current task.
func (a *ExecuteSubtask) findPriorSiblingIDs(
	parentConfig *task.Config,
	currentTaskID string,
	log logger.Logger,
) []string {
	var ids []string
	for i := range parentConfig.Tasks {
		child := parentConfig.Tasks[i]
		log.Debug("Checking sibling task",
			"index", i,
			"sibling_id", child.ID,
			"current_task_id", currentTaskID,
			"is_current", child.ID == currentTaskID)
		if child.ID == currentTaskID {
			break
		}
		ids = append(ids, child.ID)
	}
	return ids
}

// waitForSingleSibling waits until the specified sibling reaches a terminal state, ensuring
// output visibility before proceeding.
func (a *ExecuteSubtask) waitForSingleSibling(
	ctx context.Context,
	parentStateID core.ID,
	siblingID string,
	currentTaskID string,
	log logger.Logger,
	pollInterval time.Duration,
	pollTimeout time.Duration,
) error {
	deadline := time.Now().Add(pollTimeout)
	for {
		state, err := a.taskRepo.GetChildByTaskID(ctx, parentStateID, siblingID)
		if err != nil {
			if errors.Is(err, store.ErrTaskNotFound) && time.Now().Before(deadline) {
				time.Sleep(pollInterval)
				continue
			}
			return fmt.Errorf("failed to query sibling %s: %w", siblingID, err)
		}
		switch state.Status {
		case core.StatusFailed:
			log.Debug("Sibling task failed; continuing",
				"sibling_id", siblingID, "current_task", currentTaskID)
			return nil
		case core.StatusSuccess:
			if state.Output != nil {
				log.Debug("Sibling task finished with output; continuing",
					"sibling_id", siblingID, "current_task", currentTaskID)
				return nil
			}
			log.Debug("Sibling succeeded but output not yet visible",
				"sibling_id", siblingID, "current_task", currentTaskID)
		default:
			log.Debug("Sibling task still running",
				"sibling_id", siblingID, "status", state.Status)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for sibling %s to complete", siblingID)
		}
		time.Sleep(pollInterval)
	}
}
