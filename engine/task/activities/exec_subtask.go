package activities

import (
	"context"
	"errors"
	"fmt"
	"time"

	crand "crypto/rand"
	"math/big"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/task2"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/sethvargo/go-retry"
	"go.temporal.io/sdk/activity"
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
	workflowRepo   workflow.Repository
	taskRepo       task.Repository
	configStore    services.ConfigStore
	projectConfig  *project.Config
	usageMetrics   usage.Metrics
}

// NewExecuteSubtask creates and returns an ExecuteSubtask wired with the provided dependencies.
// The returned ExecuteSubtask is ready to run individual subtasks within a workflow execution.
// executeTaskUC is created without a memory manager (subtasks do not get task-local memory)
// but retains templating support via the provided templateEngine.
func NewExecuteSubtask(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	runtime runtime.Runtime,
	configStore services.ConfigStore,
	task2Factory task2.Factory,
	templateEngine *tplengine.TemplateEngine,
	projectConfig *project.Config,
	usageMetrics usage.Metrics,
	toolEnvironment toolenv.Environment,
) *ExecuteSubtask {
	return &ExecuteSubtask{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		executeTaskUC: uc.NewExecuteTask(
			runtime,
			workflowRepo,
			nil,            // Subtasks don't need memory manager
			templateEngine, // Ensure templating is available to subtasks
			nil,
			toolEnvironment,
		),
		task2Factory:   task2Factory,
		templateEngine: templateEngine,
		workflowRepo:   workflowRepo,
		taskRepo:       taskRepo,
		configStore:    configStore,
		projectConfig:  projectConfig,
		usageMetrics:   usageMetrics,
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
	ctx context.Context,
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
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
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
	ctx, finalizeCollector := a.attachUsageCollector(ctx, taskState)
	status := core.StatusFailed
	defer func() { finalizeCollector(status) }()
	output, executionError := a.executeTaskUC.Execute(ctx, &uc.ExecuteTaskInput{
		TaskConfig:     taskConfig,
		WorkflowState:  workflowState,
		WorkflowConfig: workflowConfig,
		ProjectConfig:  a.projectConfig,
	})
	if executionError == nil {
		status = core.StatusSuccess
	}
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

// attachUsageCollector adds a usage collector to the context and provides a finalize callback.
func (a *ExecuteSubtask) attachUsageCollector(
	ctx context.Context,
	taskState *task.State,
) (context.Context, func(core.StatusType)) {
	collector := usage.NewCollector(a.usageMetrics, usage.Metadata{
		Component:      taskState.Component,
		WorkflowExecID: taskState.WorkflowExecID,
		TaskExecID:     taskState.TaskExecID,
		AgentID:        taskState.AgentID,
	})
	ctx = usage.ContextWithCollector(ctx, collector)
	return ctx, func(status core.StatusType) {
		finalized, err := collector.Finalize(ctx, status)
		if err != nil {
			logger.FromContext(ctx).Warn(
				"Failed to aggregate usage for subtask execution",
				"error", err,
				"task_exec_id", taskState.TaskExecID.String(),
			)
			return
		}
		a.persistUsageSummary(ctx, taskState, finalized)
	}
}

func (a *ExecuteSubtask) persistUsageSummary(ctx context.Context, state *task.State, finalized *usage.Finalized) {
	if a == nil || state == nil || finalized == nil || finalized.Summary == nil {
		return
	}
	taskSummary := finalized.Summary.CloneWithSource(usage.SourceTask)
	if taskSummary == nil || len(taskSummary.Entries) == 0 {
		return
	}
	log := logger.FromContext(ctx)
	if err := a.taskRepo.MergeUsage(ctx, state.TaskExecID, taskSummary); err != nil {
		log.Warn(
			"Failed to merge task usage", "task_exec_id", state.TaskExecID.String(), "error", err,
		)
	}
	if a.workflowRepo != nil && !state.WorkflowExecID.IsZero() {
		workflowSummary := finalized.Summary.CloneWithSource(usage.SourceWorkflow)
		if err := a.workflowRepo.MergeUsage(ctx, state.WorkflowExecID, workflowSummary); err != nil {
			log.Warn(
				"Failed to merge workflow usage", "workflow_exec_id", state.WorkflowExecID.String(), "error", err,
			)
		}
	}
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
	priorSiblingIDs := a.findPriorSiblingIDs(ctx, parentConfig, currentTaskID)
	if len(priorSiblingIDs) == 0 {
		log.Debug("First child task - nothing to wait for",
			"current_task_id", currentTaskID)
		return nil // first child — nothing to wait for
	}
	log.Debug("Found prior siblings to wait for",
		"current_task_id", currentTaskID,
		"prior_siblings", priorSiblingIDs)

	// Poll with context-aware waits, Temporal heartbeats, and small jitter
	// to reduce contention across many activities and allow prompt cancellation.
	const (
		pollInterval = 200 * time.Millisecond
		pollTimeout  = 30 * time.Second
	)

	// Poll each prior sibling until it reaches a terminal state.
	for _, siblingID := range priorSiblingIDs {
		if err := a.waitForSingleSibling(
			ctx, parentStateID, siblingID, currentTaskID, pollInterval, pollTimeout,
		); err != nil {
			return err
		}
		// Respect cancellation between siblings
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// findPriorSiblingIDs returns the IDs of siblings that come before the current task.
func (a *ExecuteSubtask) findPriorSiblingIDs(
	ctx context.Context,
	parentConfig *task.Config,
	currentTaskID string,
) []string {
	log := logger.FromContext(ctx)
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

// randomJitter returns a random duration in [0, max).
// Uses crypto/rand to avoid predictable seeding and satisfy security linters.
func randomJitter(m time.Duration) time.Duration {
	if m <= 0 {
		return 0
	}
	n := big.NewInt(int64(m))
	r, err := crand.Int(crand.Reader, n)
	if err != nil {
		return 0
	}
	return time.Duration(r.Int64())
}

// waitForSingleSibling waits until the specified sibling reaches a terminal state, ensuring
// output visibility before proceeding.
func (a *ExecuteSubtask) waitForSingleSibling(
	ctx context.Context,
	parentStateID core.ID,
	siblingID string,
	currentTaskID string,
	pollInterval time.Duration,
	pollTimeout time.Duration,
) error {
	log := logger.FromContext(ctx)
	// Small jitter to reduce thundering herd when many activities poll together.
	// Use crypto/rand-based jitter to avoid predictability and satisfy security linters.
	deadline := time.Now().Add(pollTimeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		state, err := a.taskRepo.GetChildByTaskID(ctx, parentStateID, siblingID)
		if err != nil {
			if errors.Is(err, store.ErrTaskNotFound) && time.Now().Before(deadline) {
				// Heartbeat so Temporal can detect cancellations and we don't hold a worker slot silently.
				activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting for sibling %s to appear", siblingID))
				// Add up to 20% jitter.
				jitter := randomJitter(pollInterval / 5)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(pollInterval + jitter):
				}
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
		// Heartbeat during waits so server side can observe liveness and cancellations.
		activity.RecordHeartbeat(
			ctx,
			fmt.Sprintf("waiting for sibling %s to complete (status=%s)", siblingID, state.Status),
		)
		// Add up to 20% jitter to reduce contention.
		jitter := randomJitter(pollInterval / 5)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval + jitter):
		}
	}
}
