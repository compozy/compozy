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
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
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
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/sethvargo/go-retry"
	"go.temporal.io/sdk/activity"
)

const ExecuteSubtaskLabel = "ExecuteSubtask"

const (
	defaultChildStateRetryMax  uint64        = 5
	defaultChildStateRetryBase time.Duration = 50 * time.Millisecond
	defaultSiblingPollInterval               = 200 * time.Millisecond
	defaultSiblingPollTimeout                = 30 * time.Second
	defaultStreamMaxChunks                   = 100
)

type ExecuteSubtaskInput struct {
	WorkflowID     string  `json:"workflow_id"`
	WorkflowExecID core.ID `json:"workflow_exec_id"`
	ParentStateID  core.ID `json:"parent_state_id"` // was *task.State
	TaskExecID     string  `json:"task_exec_id"`
}

type ExecuteSubtask struct {
	loadWorkflowUC  *uc.LoadWorkflow
	executeTaskUC   *uc.ExecuteTask
	task2Factory    task2.Factory
	templateEngine  *tplengine.TemplateEngine
	workflowRepo    workflow.Repository
	taskRepo        task.Repository
	configStore     services.ConfigStore
	projectConfig   *project.Config
	usageMetrics    usage.Metrics
	streamPublisher services.StreamPublisher
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
	providerMetrics providermetrics.Recorder,
	toolEnvironment toolenv.Environment,
	streamPublisher services.StreamPublisher,
) *ExecuteSubtask {
	return &ExecuteSubtask{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		executeTaskUC: uc.NewExecuteTask(
			runtime,
			workflowRepo,
			nil,            // Subtasks don't need memory manager
			templateEngine, // Ensure templating is available to subtasks
			nil,
			providerMetrics,
			toolEnvironment,
		),
		task2Factory:    task2Factory,
		templateEngine:  templateEngine,
		workflowRepo:    workflowRepo,
		taskRepo:        taskRepo,
		configStore:     configStore,
		projectConfig:   projectConfig,
		usageMetrics:    usageMetrics,
		streamPublisher: streamPublisher,
	}
}

func (a *ExecuteSubtask) Run(ctx context.Context, input *ExecuteSubtaskInput) (*task.SubtaskResponse, error) {
	log := logger.FromContext(ctx)
	log.Debug("ExecuteSubtask.Run starting",
		"task_exec_id", input.TaskExecID,
		"parent_state_id", input.ParentStateID)
	_, workflowConfig, taskConfig, err := a.loadConfigs(ctx, input)
	if err != nil {
		return nil, err
	}
	log.Debug("Loaded task config",
		"task_id", taskConfig.ID,
		"task_type", taskConfig.Type)
	log.Debug("About to wait for prior siblings",
		"parent_state_id", input.ParentStateID,
		"current_task_id", taskConfig.ID)
	if err := a.waitForPriorSiblings(ctx, input.ParentStateID, taskConfig.ID); err != nil {
		return nil, fmt.Errorf("failed waiting for sibling tasks: %w", err)
	}
	log.Debug("Finished waiting for prior siblings")
	workflowState, _, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to refresh workflow state: %w", err)
	}
	if err := a.normalizeTask(ctx, taskConfig, workflowState, workflowConfig, &input.ParentStateID); err != nil {
		return nil, err
	}
	return a.executeAndHandleResponse(ctx, input, taskConfig, workflowState, workflowConfig)
}

func (a *ExecuteSubtask) loadConfigs(
	ctx context.Context,
	input *ExecuteSubtaskInput,
) (*workflow.State, *workflow.Config, *task.Config, error) {
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, nil, nil, err
	}
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
	normalizer, err := a.task2Factory.CreateNormalizer(ctx, taskConfig.Type)
	if err != nil {
		return fmt.Errorf("failed to create subtask normalizer: %w", err)
	}
	contextBuilder, err := shared.NewContextBuilderWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to create context builder: %w", err)
	}
	normContext := contextBuilder.BuildContextForTaskInstance(
		ctx,
		workflowState,
		workflowConfig,
		taskConfig,
		parentStateID,
	)
	if err := normalizer.Normalize(ctx, taskConfig, normContext); err != nil {
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
	status, err = a.applySubtaskExecutionResult(ctx, taskState, output, executionError)
	if err != nil {
		return nil, err
	}
	if executionError == nil {
		a.publishTextChunks(ctx, taskConfig, taskState)
	}
	result, err := a.handleSubtaskResponse(ctx, taskConfig, workflowState, workflowConfig, taskState, executionError)
	if err != nil {
		return nil, fmt.Errorf("failed to handle subtask response: %w", err)
	}
	if taskState.Status != "" {
		status = taskState.Status
	}
	subtaskResponse := a.buildSubtaskResponse(result)
	if executionError != nil {
		return subtaskResponse, executionError
	}
	return subtaskResponse, nil
}

func (a *ExecuteSubtask) publishTextChunks(ctx context.Context, cfg *task.Config, state *task.State) {
	if a == nil || a.streamPublisher == nil || cfg == nil || state == nil || ctx == nil {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}
	log := logger.FromContext(ctx)
	limit := resolveStreamChunkLimit(ctx)
	safeCtx := services.WithStreamChunkLimit(ctx, limit)
	defer func() {
		if r := recover(); r != nil {
			if log != nil {
				log.Warn("Recovered from stream chunk publish panic",
					"task_exec_id", state.TaskExecID.String(),
					"panic", r,
				)
			}
		}
	}()
	a.streamPublisher.Publish(safeCtx, cfg, state)
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
	if state == nil || finalized == nil || finalized.Summary == nil {
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

func (a *ExecuteSubtask) applySubtaskExecutionResult(
	ctx context.Context,
	taskState *task.State,
	output *core.Output,
	executionError error,
) (core.StatusType, error) {
	if executionError != nil {
		taskState.Status = core.StatusFailed
		taskState.Output = nil
		if coreErr, ok := executionError.(*core.Error); ok {
			taskState.Error = coreErr
		} else {
			taskState.Error = core.NewError(executionError, "EXECUTION_ERROR", nil)
		}
	} else {
		taskState.Status = core.StatusSuccess
		taskState.Output = output
		taskState.Error = nil
	}
	taskState.UpdatedAt = time.Now()
	if err := a.taskRepo.UpsertState(ctx, taskState); err != nil {
		return taskState.Status, fmt.Errorf("failed to persist task output and status: %w", err)
	}
	return taskState.Status, nil
}

func (a *ExecuteSubtask) handleSubtaskResponse(
	ctx context.Context,
	taskConfig *task.Config,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskState *task.State,
	executionError error,
) (*shared.ResponseOutput, error) {
	handler, err := a.task2Factory.CreateResponseHandler(ctx, taskConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to create subtask response handler: %w", err)
	}
	responseInput := &shared.ResponseInput{
		TaskConfig:     taskConfig,
		TaskState:      taskState,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
		ExecutionError: executionError,
	}
	return handler.HandleResponse(ctx, responseInput)
}

func (a *ExecuteSubtask) buildSubtaskResponse(result *shared.ResponseOutput) *task.SubtaskResponse {
	converter := NewResponseConverter()
	mainTaskResponse := converter.ConvertToMainTaskResponse(result)
	return &task.SubtaskResponse{
		State: mainTaskResponse.State,
	}
}

// getChildStateWithRetry retrieves child state with exponential backoff retry
func (a *ExecuteSubtask) getChildStateWithRetry(
	ctx context.Context,
	parentStateID core.ID,
	taskID string,
) (*task.State, error) {
	var taskState *task.State
	maxRetries, baseBackoff := resolveChildStateRetry(ctx)
	err := retry.Do(
		ctx,
		retry.WithMaxRetries(maxRetries, retry.NewExponential(baseBackoff)),
		func(ctx context.Context) error {
			var err error
			taskState, err = a.getChildState(ctx, parentStateID, taskID)
			if err != nil {
				if !errors.Is(err, store.ErrTaskNotFound) {
					return fmt.Errorf("failed to get child state: %w", err)
				}
				return retry.RetryableError(err)
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("child state for task %s not found after retries: %w", taskID, err)
	}
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
	parentConfig := a.loadParentConfigForOrdering(ctx, parentStateID, log)
	if parentConfig == nil {
		return nil
	}
	priorSiblingIDs := a.findPriorSiblingIDs(ctx, parentConfig, currentTaskID)
	if len(priorSiblingIDs) == 0 {
		log.Debug("First child task - nothing to wait for", "current_task_id", currentTaskID)
		return nil
	}
	log.Debug("Found prior siblings to wait for",
		"current_task_id", currentTaskID,
		"prior_siblings", priorSiblingIDs)
	return a.waitOnSiblingSequence(ctx, parentStateID, currentTaskID, priorSiblingIDs)
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
	deadline := time.Now().Add(pollTimeout)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		state, err := a.taskRepo.GetChildByTaskID(ctx, parentStateID, siblingID)
		if a.shouldRetrySiblingQuery(err, deadline) {
			if err := a.sleepWithHeartbeat(ctx, siblingID, pollInterval); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to query sibling %s: %w", siblingID, err)
		}
		if a.siblingReachedTerminal(state, log, siblingID, currentTaskID) {
			return nil
		}
		if err := a.waitForNextPoll(ctx, siblingID, state.Status, deadline, pollInterval); err != nil {
			return err
		}
	}
}

// loadParentConfigForOrdering retrieves the parent config and logs diagnostic messages.
func (a *ExecuteSubtask) loadParentConfigForOrdering(
	ctx context.Context,
	parentStateID core.ID,
	log logger.Logger,
) *task.Config {
	parentConfig, err := a.configStore.Get(ctx, parentStateID.String())
	if err != nil {
		log.Warn("could not load parent task config; proceeding without sibling ordering",
			"parent_state_id", parentStateID, "error", err)
		return nil
	}
	if parentConfig == nil || len(parentConfig.Tasks) == 0 {
		log.Debug("No parent config or no sibling tasks found",
			"parent_state_id", parentStateID,
			"parent_config_nil", parentConfig == nil)
		return nil
	}
	log.Debug("Found parent config with tasks",
		"parent_state_id", parentStateID,
		"num_tasks", len(parentConfig.Tasks),
		"parent_type", parentConfig.Type)
	return parentConfig
}

// waitOnSiblingSequence iterates sequentially through sibling IDs ensuring ordering guarantees.
func (a *ExecuteSubtask) waitOnSiblingSequence(
	ctx context.Context,
	parentStateID core.ID,
	currentTaskID string,
	priorSiblingIDs []string,
) error {
	pollInterval, pollTimeout := resolveSiblingWaitSettings(ctx)
	for _, siblingID := range priorSiblingIDs {
		if err := a.waitForSingleSibling(
			ctx,
			parentStateID,
			siblingID,
			currentTaskID,
			pollInterval,
			pollTimeout,
		); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// shouldRetrySiblingQuery determines whether to retry fetching a sibling state.
func (a *ExecuteSubtask) shouldRetrySiblingQuery(err error, deadline time.Time) bool {
	return err != nil && errors.Is(err, store.ErrTaskNotFound) && time.Now().Before(deadline)
}

// sleepWithHeartbeat waits for the next polling cycle while heartbeating for Temporal.
func (a *ExecuteSubtask) sleepWithHeartbeat(
	ctx context.Context,
	siblingID string,
	pollInterval time.Duration,
) error {
	activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting for sibling %s to appear", siblingID))
	jitter := randomJitter(pollInterval / 5)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(pollInterval + jitter):
		return nil
	}
}

// siblingReachedTerminal returns true when the sibling reached a terminal state with visible data.
func (a *ExecuteSubtask) siblingReachedTerminal(
	state *task.State,
	log logger.Logger,
	siblingID string,
	currentTaskID string,
) bool {
	switch state.Status {
	case core.StatusFailed:
		log.Debug("Sibling task failed; continuing",
			"sibling_id", siblingID, "current_task", currentTaskID)
		return true
	case core.StatusSuccess:
		if state.Output != nil {
			log.Debug("Sibling task finished with output; continuing",
				"sibling_id", siblingID, "current_task", currentTaskID)
			return true
		}
		log.Debug("Sibling succeeded but output not yet visible",
			"sibling_id", siblingID, "current_task", currentTaskID)
	default:
		log.Debug("Sibling task still running",
			"sibling_id", siblingID, "status", state.Status)
	}
	return false
}

// waitForNextPoll manages repeated polling including timeouts and heartbeats.
func (a *ExecuteSubtask) waitForNextPoll(
	ctx context.Context,
	siblingID string,
	status core.StatusType,
	deadline time.Time,
	pollInterval time.Duration,
) error {
	if time.Now().After(deadline) {
		return fmt.Errorf("timeout waiting for sibling %s to complete", siblingID)
	}
	activity.RecordHeartbeat(
		ctx,
		fmt.Sprintf("waiting for sibling %s to complete (status=%s)", siblingID, status),
	)
	jitter := randomJitter(pollInterval / 5)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(pollInterval + jitter):
		return nil
	}
}

func resolveChildStateRetry(ctx context.Context) (uint64, time.Duration) {
	attempts := defaultChildStateRetryMax
	base := defaultChildStateRetryBase
	if cfg := config.FromContext(ctx); cfg != nil {
		if value := cfg.Tasks.Retry.ChildState.MaxAttempts; value > 0 {
			attempts = uint64(value)
		}
		if value := cfg.Tasks.Retry.ChildState.BaseBackoff; value > 0 {
			base = value
		}
	}
	return attempts, base
}

func resolveSiblingWaitSettings(ctx context.Context) (time.Duration, time.Duration) {
	interval := defaultSiblingPollInterval
	timeout := defaultSiblingPollTimeout
	if cfg := config.FromContext(ctx); cfg != nil {
		if value := cfg.Tasks.Wait.Siblings.PollInterval; value > 0 {
			interval = value
		}
		if value := cfg.Tasks.Wait.Siblings.Timeout; value > 0 {
			timeout = value
		}
	}
	return interval, timeout
}

func resolveStreamChunkLimit(ctx context.Context) int {
	limit := defaultStreamMaxChunks
	if cfg := config.FromContext(ctx); cfg != nil {
		if value := cfg.Tasks.Stream.MaxChunks; value >= 0 {
			limit = value
		}
	}
	return limit
}
