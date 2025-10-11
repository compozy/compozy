package wfrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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
	Workflow *workflow.State `json:"workflow"`
	Output   *core.Output    `json:"output,omitempty"`
	ExecID   string          `json:"exec_id"          example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
}

// executeWorkflowSync handles POST /workflows/{workflow_id}/executions/sync.
//
//	@Summary		Execute workflow synchronously
//	@Description	Execute a workflow and wait for completion within the provided timeout.
//	@Tags			workflows
//	@Accept			json
//	@Produce		json
//	@Param			workflow_id	path		string	true	"Workflow ID"	example("data-processing")
//	@Param			X-Correlation-ID	header		string	false	"Optional correlation ID for request tracing"
//	@Param			payload	body	wfrouter.WorkflowSyncRequest	true	"Execution request"
//	@Success		200	{object}	router.Response{data=wfrouter.WorkflowSyncResponse}	"Workflow execution completed"
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
	var inputPtr *core.Input
	if req.Input != nil {
		copyInput := req.Input
		inputPtr = &copyInput
	}
	triggered, err := runner.TriggerWorkflow(c.Request.Context(), workflowID, inputPtr, req.TaskID)
	if err != nil {
		status := respondWorkflowStartError(c, workflowID, err)
		recordError(status)
		return
	}
	execID := triggered.WorkflowExecID
	log := logger.FromContext(c.Request.Context())
	log.Info("Workflow execution started", "workflow_id", workflowID, "exec_id", execID.String())
	deadline := time.Duration(req.Timeout) * workflowTimeoutUnit
	stateResult, timedOut, pollErr := waitForWorkflowCompletion(c.Request.Context(), repo, execID, deadline)
	if pollErr != nil {
		recordError(http.StatusInternalServerError)
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to monitor workflow execution", pollErr)
		return
	}
	if timedOut {
		outcome = monitoring.ExecutionOutcomeTimeout
		status := respondWorkflowTimeout(c, repo, workflowID, execID, stateResult, metrics)
		recordError(status)
		return
	}
	payload := gin.H{
		"workflow": stateResult,
		"output":   stateResult.Output,
		"exec_id":  execID.String(),
	}
	outcome = monitoring.ExecutionOutcomeSuccess
	router.RespondOK(c, "workflow execution completed", payload)
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
) (*workflow.State, bool, error) {
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	interval := workflowPollInitialBackoff
	attempt := 0
	var lastState *workflow.State
	for {
		if pollCtx.Err() != nil {
			state, err := finalWorkflowState(ctx, repo, execID, lastState)
			return state, true, err
		}
		state, err := repo.GetState(pollCtx, execID)
		if err != nil {
			if !isIgnorablePollError(err) {
				return lastState, false, err
			}
		} else if state != nil {
			lastState = state
			if isWorkflowTerminal(state.Status) {
				return state, false, nil
			}
		}
		wait := applyWorkflowJitter(interval, execID, attempt)
		interval = nextWorkflowBackoff(interval)
		attempt++
		if !waitForNextWorkflowPoll(pollCtx, wait) {
			state, finalErr := finalWorkflowState(ctx, repo, execID, lastState)
			return state, true, finalErr
		}
	}
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

func waitForNextWorkflowPoll(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
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

func applyWorkflowJitter(base time.Duration, execID core.ID, attempt int) time.Duration {
	if base <= 0 {
		return base
	}
	span := base / 10
	if span <= 0 {
		span = time.Millisecond
	}
	id := execID.String() + strconv.Itoa(attempt)
	spanNanos := int64(span)
	rangeSize := spanNanos*2 + 1
	if rangeSize <= 0 {
		rangeSize = 1
	}
	hashVal := int64(0)
	for i := 0; i < len(id); i++ {
		hashVal = (hashVal*33 + int64(id[i])) % rangeSize
	}
	rawHash := max(hashVal, 0)
	offset := (rawHash % rangeSize) - spanNanos
	result := base + time.Duration(offset)
	if result < time.Millisecond {
		return time.Millisecond
	}
	return result
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
	log := logger.FromContext(ctx)
	payload := gin.H{"exec_id": execID.String()}
	if state == nil && repo != nil {
		latest, err := repo.GetState(context.WithoutCancel(ctx), execID)
		if err == nil && latest != nil {
			state = latest
		} else if err != nil && !errors.Is(err, store.ErrWorkflowNotFound) {
			log.Warn(
				"Failed to load workflow state after timeout",
				"workflow_id",
				workflowID,
				"exec_id",
				execID.String(),
				"error",
				err,
			)
		}
	}
	if state != nil {
		payload["state"] = state
	}
	log.Warn("Workflow execution timed out", "workflow_id", workflowID, "exec_id", execID.String())
	resp := router.Response{
		Status:  http.StatusRequestTimeout,
		Message: "execution timeout",
		Data:    payload,
		Error: &router.ErrorInfo{
			Code:    router.ErrRequestTimeoutCode,
			Message: "execution timeout",
			Details: context.DeadlineExceeded.Error(),
		},
	}
	c.JSON(http.StatusRequestTimeout, resp)
	if metrics != nil {
		metrics.RecordTimeout(ctx, monitoring.ExecutionKindWorkflow)
	}
	return http.StatusRequestTimeout
}
