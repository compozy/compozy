package wfrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/engine/workflow/uc"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	workflowSyncDefaultTimeoutSeconds = 60
	workflowSyncMaxTimeoutSeconds     = 300
	workflowSyncInitialBackoff        = 200 * time.Millisecond
	workflowSyncMaxBackoff            = 5 * time.Second
)

var (
	workflowPollInitialBackoff = workflowSyncInitialBackoff
	workflowPollMaxBackoff     = workflowSyncMaxBackoff
	workflowTimeoutUnit        = time.Second
)

// WorkflowSyncRequest represents the request body for synchronous workflow execution.
type WorkflowSyncRequest struct {
	Input  core.Input `json:"input"`
	TaskID string     `json:"task_id"`
	// Timeout in seconds for synchronous execution.
	Timeout int `json:"timeout"`
}

// WorkflowSyncResponse represents the response body for synchronous workflow execution.
type WorkflowSyncResponse struct {
	Workflow *WorkflowExecutionDTO `json:"workflow"`
	Output   *core.Output          `json:"output,omitempty"`
	ExecID   string                `json:"exec_id"          example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
}

// executeWorkflowSync handles POST /workflows/{workflow_id}/executions/sync.
//
//	@Summary		Execute workflow synchronously
//	@Description	Execute a workflow and wait for completion within the provided timeout. The response includes a workflow.usage field containing aggregated LLM token counts grouped by provider and model.
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string	true	"Workflow ID"	example("data-processing")
//	@Param			X-Correlation-ID	header		string	false	"Optional correlation ID for request tracing"
//	@Param			payload	body	wfrouter.WorkflowSyncRequest	true	"Execution request"
//	@Success		200	{object}	router.Response{data=wfrouter.WorkflowSyncResponse}	"Workflow execution completed. The data.workflow.usage field is an array of usage entries with prompt_tokens, completion_tokens, total_tokens, and optional reasoning_tokens, cached_prompt_tokens, input_audio_tokens, and output_audio_tokens per provider/model combination."
//	@Failure		400	{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404	{object}	router.Response{error=router.ErrorInfo}	"Workflow not found"
//	@Failure		408	{object}	router.Response{error=router.ErrorInfo}	"Execution timeout"
//	@Failure		409	{object}	router.Response{error=router.ErrorInfo}	"Duplicate request"
//	@Failure		503	{object}	router.Response{error=router.ErrorInfo}	"Worker unavailable"
//	@Failure		500	{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/workflows/{workflow_id}/executions/sync [post]
func executeWorkflowSync(c *gin.Context) {
	workflowID := router.GetWorkflowID(c)
	if workflowID == "" {
		return
	}
	state := router.GetAppState(c)
	if state == nil {
		return
	}
	metrics, finalizeMetrics, recordError := router.SyncExecutionMetricsScope(
		c,
		state,
		monitoring.ExecutionKindWorkflow,
	)
	outcome := monitoring.ExecutionOutcomeError
	defer func() { finalizeMetrics(outcome) }()
	req := parseWorkflowSyncRequest(c)
	if req == nil {
		recordError(http.StatusBadRequest)
		return
	}
	repo, runner, ok := prepareWorkflowSync(c, state, workflowID, recordError)
	if !ok {
		return
	}
	execID, ok := triggerWorkflowSync(c, runner, workflowID, workflowInputPointer(req.Input), req.TaskID, recordError)
	if !ok {
		return
	}
	log := logger.FromContext(c.Request.Context())
	log.Info("Workflow execution started", "workflow_id", workflowID, "exec_id", execID.String())
	deadline := workflowDeadline(req.Timeout)
	stateResult, timedOut, pollErr := waitForWorkflowCompletion(
		c.Request.Context(),
		repo,
		execID,
		deadline,
		workflowID,
		metrics,
	)
	newOutcome, handled := handleWorkflowCompletionResult(
		c,
		repo,
		workflowID,
		execID,
		stateResult,
		timedOut,
		pollErr,
		metrics,
		recordError,
	)
	if handled {
		outcome = newOutcome
		return
	}
	outcome = newOutcome
	summary := router.NewUsageSummary(stateResult.Usage)
	response := WorkflowSyncResponse{
		Workflow: newWorkflowExecutionDTO(stateResult, summary),
		Output:   stateResult.Output,
		ExecID:   execID.String(),
	}
	router.RespondOK(c, "workflow execution completed", response)
}

func triggerWorkflowSync(
	c *gin.Context,
	runner router.WorkflowRunner,
	workflowID string,
	input *core.Input,
	taskID string,
	recordError func(int),
) (core.ID, bool) {
	triggered, err := runner.TriggerWorkflow(c.Request.Context(), workflowID, input, taskID)
	if err != nil {
		status := respondWorkflowStartError(c, workflowID, err)
		recordError(status)
		var zeroID core.ID
		return zeroID, false
	}
	return triggered.WorkflowExecID, true
}

func workflowInputPointer(input core.Input) *core.Input {
	if input == nil {
		return nil
	}
	copyInput := input
	return &copyInput
}

func workflowDeadline(timeoutSeconds int) time.Duration {
	return time.Duration(timeoutSeconds) * workflowTimeoutUnit
}

func handleWorkflowCompletionResult(
	c *gin.Context,
	repo workflow.Repository,
	workflowID string,
	execID core.ID,
	stateResult *workflow.State,
	timedOut bool,
	pollErr error,
	metrics *monitoring.ExecutionMetrics,
	recordError func(int),
) (string, bool) {
	if pollErr != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to monitor workflow execution", pollErr)
		return monitoring.ExecutionOutcomeError, true
	}
	if timedOut {
		status := respondWorkflowTimeout(c, repo, workflowID, execID, stateResult, metrics)
		recordError(status)
		return monitoring.ExecutionOutcomeTimeout, true
	}
	return monitoring.ExecutionOutcomeSuccess, false
}

func parseWorkflowSyncRequest(c *gin.Context) *WorkflowSyncRequest {
	req := router.GetRequestBody[WorkflowSyncRequest](c)
	if req == nil {
		return nil
	}
	if req.Timeout < 0 {
		reqErr := router.NewRequestError(http.StatusBadRequest, "timeout must be non-negative", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}
	if req.Timeout == 0 {
		req.Timeout = workflowSyncDefaultTimeoutSeconds
	}
	if req.Timeout > workflowSyncMaxTimeoutSeconds {
		message := fmt.Sprintf("timeout cannot exceed %d seconds", workflowSyncMaxTimeoutSeconds)
		reqErr := router.NewRequestError(http.StatusBadRequest, message, nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil
	}
	return req
}

func ensureWorkflowExists(c *gin.Context, store resources.ResourceStore, project string, workflowID string) bool {
	getUC := uc.NewGet(store)
	_, err := getUC.Execute(c.Request.Context(), &uc.GetInput{Project: project, ID: workflowID})
	if err == nil {
		return true
	}
	if errors.Is(err, uc.ErrNotFound) {
		reqErr := router.NewRequestError(http.StatusNotFound, "workflow not found", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return false
	}
	router.RespondWithServerError(c, router.ErrInternalCode, "failed to load workflow", err)
	return false
}

func prepareWorkflowSync(
	c *gin.Context,
	state *appstate.State,
	workflowID string,
	recordError func(int),
) (workflow.Repository, router.WorkflowRunner, bool) {
	repo := router.ResolveWorkflowRepository(c, state)
	if repo == nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(
			c,
			router.ErrInternalCode,
			"workflow repository not available",
			fmt.Errorf("workflow repository not configured"),
		)
		return nil, nil, false
	}
	store, ok := router.GetResourceStore(c)
	if !ok {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return nil, nil, false
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return nil, nil, false
	}
	if !ensureWorkflowExists(c, store, project, workflowID) {
		recordError(router.StatusOrFallback(c, http.StatusInternalServerError))
		return nil, nil, false
	}
	runner := router.ResolveWorkflowRunner(c, state)
	if runner == nil {
		recordError(http.StatusServiceUnavailable)
		reqErr := router.NewRequestError(http.StatusServiceUnavailable, router.ErrMsgWorkerNotRunning, nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, nil, false
	}
	return repo, runner, true
}

func respondWorkflowStartError(c *gin.Context, workflowID string, err error) int {
	if strings.Contains(strings.ToLower(err.Error()), "workflow not found") {
		reqErr := router.NewRequestError(http.StatusNotFound, "workflow not found", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return reqErr.StatusCode
	}
	reqErr := router.WorkflowExecutionError(workflowID, fmt.Sprintf("failed to trigger workflow: %s", workflowID), err)
	router.RespondWithError(c, reqErr.StatusCode, reqErr)
	return reqErr.StatusCode
}

func waitForWorkflowCompletion(
	ctx context.Context,
	repo workflow.Repository,
	execID core.ID,
	timeout time.Duration,
	workflowID string,
	metrics *monitoring.ExecutionMetrics,
) (*workflow.State, bool, error) {
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tracker := newWorkflowPollTracker(ctx, metrics, workflowID)
	defer tracker.finish()
	timer := time.NewTimer(0)
	<-timer.C
	defer timer.Stop()
	return runWorkflowPollLoop(ctx, pollCtx, repo, execID, timer, tracker)
}

func runWorkflowPollLoop(
	ctx context.Context,
	pollCtx context.Context,
	repo workflow.Repository,
	execID core.ID,
	timer *time.Timer,
	tracker *workflowPollTracker,
) (*workflow.State, bool, error) {
	interval := workflowPollInitialBackoff
	execIDStr := execID.String()
	var lastState *workflow.State
	for {
		if pollCtx.Err() != nil {
			tracker.setOutcome(monitoring.WorkflowPollOutcomeTimeout)
			state, err := finalWorkflowState(ctx, repo, execID, lastState)
			return state, true, err
		}
		state, err := repo.GetState(pollCtx, execID)
		tracker.increment()
		if err != nil {
			if !isIgnorablePollError(err) {
				tracker.setOutcome(monitoring.WorkflowPollOutcomeError)
				return lastState, false, err
			}
		} else if state != nil {
			lastState = state
			if isWorkflowTerminal(state.Status) {
				tracker.setOutcome(monitoring.WorkflowPollOutcomeCompleted)
				return state, false, nil
			}
		}
		wait := applyWorkflowJitter(interval, execIDStr, tracker.attemptIndex())
		interval = nextWorkflowBackoff(interval)
		if !waitForNextWorkflowPoll(pollCtx, timer, wait) {
			tracker.setOutcome(monitoring.WorkflowPollOutcomeTimeout)
			state, finalErr := finalWorkflowState(ctx, repo, execID, lastState)
			return state, true, finalErr
		}
	}
}

type workflowPollTracker struct {
	metrics    *monitoring.ExecutionMetrics
	ctx        context.Context
	workflowID string
	start      time.Time
	outcome    string
	count      int
}

func newWorkflowPollTracker(
	ctx context.Context,
	metrics *monitoring.ExecutionMetrics,
	workflowID string,
) *workflowPollTracker {
	return &workflowPollTracker{
		metrics:    metrics,
		ctx:        ctx,
		workflowID: workflowID,
		start:      time.Now(),
		outcome:    monitoring.WorkflowPollOutcomeError,
	}
}

func (t *workflowPollTracker) increment() {
	t.count++
}

func (t *workflowPollTracker) setOutcome(outcome string) {
	t.outcome = outcome
}

func (t *workflowPollTracker) attemptIndex() int {
	return t.count - 1
}

func (t *workflowPollTracker) finish() {
	if t.metrics == nil {
		return
	}
	t.metrics.RecordWorkflowPollDuration(t.ctx, t.workflowID, time.Since(t.start))
	t.metrics.RecordWorkflowPolls(t.ctx, t.workflowID, t.count, t.outcome)
}

func isIgnorablePollError(err error) bool {
	return errors.Is(err, store.ErrWorkflowNotFound) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func nextWorkflowBackoff(current time.Duration) time.Duration {
	maxBackoff := workflowPollMaxBackoff
	if current >= maxBackoff {
		return maxBackoff
	}
	next := current * 2
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

func waitForNextWorkflowPoll(ctx context.Context, timer *time.Timer, delay time.Duration) bool {
	if delay <= 0 {
		delay = time.Millisecond
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		return false
	case <-timer.C:
		return true
	}
}

func finalWorkflowState(
	ctx context.Context,
	repo workflow.Repository,
	execID core.ID,
	lastState *workflow.State,
) (*workflow.State, error) {
	state, err := repo.GetState(context.WithoutCancel(ctx), execID)
	if err != nil {
		if errors.Is(err, store.ErrWorkflowNotFound) {
			return lastState, nil
		}
		return lastState, err
	}
	if state == nil {
		return lastState, nil
	}
	return state, nil
}

func isWorkflowTerminal(status core.StatusType) bool {
	switch status {
	case core.StatusSuccess, core.StatusFailed, core.StatusTimedOut, core.StatusCanceled:
		return true
	default:
		return false
	}
}

func applyWorkflowJitter(base time.Duration, execID string, attempt int) time.Duration {
	if base <= 0 {
		return base
	}
	span := base / 10
	if span <= 0 {
		span = time.Millisecond
	}
	spanNanos := int64(span)
	rangeSize := spanNanos*2 + 1
	if rangeSize <= 0 {
		rangeSize = 1
	}
	hashVal := computeJitterHash(execID, attempt, rangeSize)
	offset := hashVal - spanNanos
	result := base + time.Duration(offset)
	if result < time.Millisecond {
		return time.Millisecond
	}
	return result
}

func computeJitterHash(execID string, attempt int, rangeSize int64) int64 {
	hashVal := int64(5381)
	for i := 0; i < len(execID); i++ {
		hashVal = djb2Step(hashVal, int64(execID[i]), rangeSize)
	}
	if attempt < 0 {
		hashVal = djb2Step(hashVal, int64('-'), rangeSize)
	} else {
		hashVal = djb2Step(hashVal, int64('+'), rangeSize)
	}
	digits := formatAttemptDigits(attempt)
	for _, d := range digits {
		hashVal = djb2Step(hashVal, int64(d), rangeSize)
	}
	return hashVal % rangeSize
}

func djb2Step(hash, value, mod int64) int64 {
	return ((hash << 5) + hash + value) % mod
}

func formatAttemptDigits(attempt int) []byte {
	value := attempt
	if value < 0 {
		value = -value
	}
	if value == 0 {
		return []byte{'0'}
	}
	digits := make([]byte, 0, 20)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return digits
}

func respondWorkflowTimeout(
	c *gin.Context,
	repo workflow.Repository,
	workflowID string,
	execID core.ID,
	state *workflow.State,
	metrics *monitoring.ExecutionMetrics,
) int {
	ctx := c.Request.Context()
	state = resolveTimeoutState(ctx, repo, workflowID, execID, state)
	payload := buildTimeoutPayload(ctx, repo, execID, state)
	logger.FromContext(ctx).Warn("Workflow execution timed out", "workflow_id", workflowID, "exec_id", execID.String())
	c.JSON(http.StatusRequestTimeout, buildTimeoutResponse(payload))
	recordWorkflowTimeoutMetric(ctx, metrics)
	return http.StatusRequestTimeout
}

func resolveTimeoutState(
	ctx context.Context,
	repo workflow.Repository,
	workflowID string,
	execID core.ID,
	state *workflow.State,
) *workflow.State {
	if state != nil || repo == nil {
		return state
	}
	latest, err := repo.GetState(context.WithoutCancel(ctx), execID)
	if err != nil {
		if !errors.Is(err, store.ErrWorkflowNotFound) {
			logger.FromContext(ctx).Warn(
				"Failed to load workflow state after timeout",
				"workflow_id",
				workflowID,
				"exec_id",
				execID.String(),
				"error",
				err,
			)
		}
		return state
	}
	if latest == nil {
		return state
	}
	return latest
}

func buildTimeoutPayload(
	ctx context.Context,
	repo workflow.Repository,
	execID core.ID,
	state *workflow.State,
) gin.H {
	payload := gin.H{"exec_id": execID.String()}
	if state != nil {
		summary := router.NewUsageSummary(state.Usage)
		payload["workflow"] = newWorkflowExecutionDTO(state, summary)
		if summary != nil {
			return payload
		}
	}
	if repo != nil {
		if summary := router.ResolveWorkflowUsageSummary(context.WithoutCancel(ctx), repo, execID); summary != nil {
			payload["usage"] = summary
		}
	}
	return payload
}

func buildTimeoutResponse(payload gin.H) router.Response {
	return router.Response{
		Status:  http.StatusRequestTimeout,
		Message: "execution timeout",
		Data:    payload,
		Error: &router.ErrorInfo{
			Code:    router.ErrRequestTimeoutCode,
			Message: "execution timeout",
			Details: context.DeadlineExceeded.Error(),
		},
	}
}

func recordWorkflowTimeoutMetric(ctx context.Context, metrics *monitoring.ExecutionMetrics) {
	if metrics == nil {
		return
	}
	metrics.RecordTimeout(ctx, monitoring.ExecutionKindWorkflow)
}
