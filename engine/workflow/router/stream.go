package wfrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	workflowStreamDefaultPoll   = 500 * time.Millisecond
	workflowStreamMinPoll       = 250 * time.Millisecond
	workflowStreamMaxPoll       = 2000 * time.Millisecond
	workflowStreamHeartbeatFreq = 15 * time.Second
	workflowStreamQueryTimeout  = 5 * time.Second
)

type workflowQueryClient interface {
	QueryWorkflow(
		ctx context.Context,
		workflowID string,
		runID string,
		queryType string,
		args ...any,
	) (converter.EncodedValue, error)
}

type temporalWorkflowQueryClient struct {
	client client.Client
}

func (t temporalWorkflowQueryClient) QueryWorkflow(
	ctx context.Context,
	workflowID string,
	runID string,
	queryType string,
	args ...any,
) (converter.EncodedValue, error) {
	if t.client == nil {
		return nil, errors.New("temporal client not configured")
	}
	return t.client.QueryWorkflow(ctx, workflowID, runID, queryType, args...)
}

func parseWorkflowExecID(c *gin.Context) (core.ID, bool) {
	execID := router.GetWorkflowExecID(c)
	if execID == "" {
		return "", false
	}
	if _, err := core.ParseID(execID.String()); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "Invalid execution ID", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return "", false
	}
	return execID, true
}

func parseWorkflowPollIntervalParam(c *gin.Context) (time.Duration, bool) {
	interval, err := parseWorkflowPollInterval(c.Query("poll_ms"))
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid poll interval", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return interval, true
}

func parseLastEventIDHeader(c *gin.Context) (int64, bool) {
	lastID, _, err := router.LastEventID(c.Request)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid Last-Event-ID header", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return lastID, true
}

func resolveWorkflowStreamContext(
	c *gin.Context,
) (workflowQueryClient, context.Context, logger.Logger, bool) {
	state, ok := ensureWorkerReady(c)
	if !ok {
		return nil, nil, nil, false
	}
	queryClient := resolveWorkflowQueryClient(state)
	if queryClient == nil {
		router.RespondProblemWithCode(
			c,
			http.StatusServiceUnavailable,
			router.ErrServiceUnavailableCode,
			"workflow query client unavailable",
		)
		return nil, nil, nil, false
	}
	ctx := c.Request.Context()
	return queryClient, ctx, logger.FromContext(ctx), true
}

// streamWorkflowExecution handles SSE streaming for workflow executions.
func streamWorkflowExecution(c *gin.Context) {
	execID, ok := parseWorkflowExecID(c)
	if !ok {
		return
	}
	pollInterval, ok := parseWorkflowPollIntervalParam(c)
	if !ok {
		return
	}
	lastEventID, ok := parseLastEventIDHeader(c)
	if !ok {
		return
	}
	queryClient, ctx, log, ok := resolveWorkflowStreamContext(c)
	if !ok {
		return
	}
	processWorkflowStream(ctx, c, execID, queryClient, pollInterval, lastEventID, log)
}

func processWorkflowStream(
	ctx context.Context,
	c *gin.Context,
	execID core.ID,
	queryClient workflowQueryClient,
	pollInterval time.Duration,
	lastEventID int64,
	log logger.Logger,
) {
	snapshot, status, err := fetchWorkflowStreamState(ctx, queryClient, execID.String())
	if err != nil {
		respondWorkflowQueryError(c, err)
		return
	}
	stream := router.StartSSE(c.Writer)
	if stream == nil {
		router.RespondProblemWithCode(
			c,
			http.StatusInternalServerError,
			router.ErrInternalCode,
			"failed to initialize stream",
		)
		return
	}
	log.Info("Workflow stream connected", "exec_id", execID, "last_event_id", lastEventID)
	lastEventID, ok := writeInitialSnapshot(stream, log, execID, snapshot, status, lastEventID)
	if !ok {
		return
	}
	finalStatus, loopErr := runWorkflowStreamLoop(
		ctx,
		stream,
		execID,
		queryClient,
		pollInterval,
		&lastEventID,
	)
	logLoopOutcome(log, execID, lastEventID, finalStatus, loopErr)
}

func writeInitialSnapshot(
	stream *router.SSEStream,
	log logger.Logger,
	execID core.ID,
	snapshot *wf.StreamState,
	status core.StatusType,
	lastEventID int64,
) (int64, bool) {
	updatedID, err := emitWorkflowEvents(stream, snapshot, lastEventID)
	if err != nil {
		log.Warn("workflow stream write failed", "exec_id", execID, "error", err)
		return lastEventID, false
	}
	lastEventID = updatedID
	if isWorkflowTerminalStatus(status) {
		log.Info("Workflow stream disconnected", "exec_id", execID, "last_event_id", lastEventID)
		return lastEventID, false
	}
	return lastEventID, true
}

func logLoopOutcome(
	log logger.Logger,
	execID core.ID,
	lastEventID int64,
	finalStatus core.StatusType,
	loopErr error,
) {
	if loopErr != nil {
		if errors.Is(loopErr, context.Canceled) {
			log.Info(
				"Workflow stream canceled",
				"exec_id", execID,
				"last_event_id", lastEventID,
			)
			return
		}
		log.Warn(
			"workflow stream terminated with error",
			"exec_id", execID,
			"error", loopErr,
			"last_event_id", lastEventID,
		)
		return
	}
	log.Info(
		"Workflow stream disconnected",
		"exec_id", execID,
		"last_event_id", lastEventID,
		"status", finalStatus,
	)
}

func resolveWorkflowQueryClient(state *appstate.State) workflowQueryClient {
	if state == nil {
		return nil
	}
	if ext, ok := state.WorkflowQueryClient(); ok {
		if client, ok := ext.(workflowQueryClient); ok && client != nil {
			return client
		}
	}
	if state.Worker != nil {
		return temporalWorkflowQueryClient{client: state.Worker.GetClient()}
	}
	return nil
}

func fetchWorkflowStreamState(
	ctx context.Context,
	client workflowQueryClient,
	workflowID string,
) (*wf.StreamState, core.StatusType, error) {
	queryCtx, cancel := context.WithTimeout(ctx, workflowStreamQueryTimeout)
	defer cancel()
	value, err := client.QueryWorkflow(queryCtx, workflowID, "", wf.StreamQueryName)
	if err != nil {
		return nil, core.StatusPending, err
	}
	var state wf.StreamState
	if err := value.Get(&state); err != nil {
		return nil, core.StatusPending, err
	}
	return &state, state.Status, nil
}

func runWorkflowStreamLoop(
	ctx context.Context,
	stream *router.SSEStream,
	execID core.ID,
	client workflowQueryClient,
	pollInterval time.Duration,
	lastEventID *int64,
) (core.StatusType, error) {
	heartbeatTicker := time.NewTicker(workflowStreamHeartbeatFreq)
	pollTicker := time.NewTicker(pollInterval)
	defer heartbeatTicker.Stop()
	defer pollTicker.Stop()
	status := core.StatusRunning
	for {
		select {
		case <-ctx.Done():
			return status, ctx.Err()
		case <-heartbeatTicker.C:
			if err := stream.WriteHeartbeat(); err != nil {
				return status, err
			}
		case <-pollTicker.C:
			snapshot, currentStatus, err := fetchWorkflowStreamState(ctx, client, execID.String())
			if err != nil {
				return status, err
			}
			updatedID, writeErr := emitWorkflowEvents(stream, snapshot, *lastEventID)
			if writeErr != nil {
				return status, writeErr
			}
			*lastEventID = updatedID
			status = currentStatus
			if isWorkflowTerminalStatus(currentStatus) {
				return status, nil
			}
		}
	}
}

func emitWorkflowEvents(stream *router.SSEStream, state *wf.StreamState, lastID int64) (int64, error) {
	if stream == nil || state == nil {
		return lastID, nil
	}
	for _, event := range state.Events {
		if event.ID <= lastID {
			continue
		}
		if err := stream.WriteEvent(event.ID, event.Type, event.Data); err != nil {
			return lastID, err
		}
		lastID = event.ID
	}
	return lastID, nil
}

func parseWorkflowPollInterval(raw string) (time.Duration, error) {
	if raw == "" {
		return workflowStreamDefaultPoll, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid poll_ms: %w", err)
	}
	if ms < int(workflowStreamMinPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be >= %d", workflowStreamMinPoll/time.Millisecond)
	}
	if ms > int(workflowStreamMaxPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be <= %d", workflowStreamMaxPoll/time.Millisecond)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func respondWorkflowQueryError(c *gin.Context, err error) {
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		reqErr := router.NewRequestError(http.StatusNotFound, "workflow execution not found", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	var invalid *serviceerror.InvalidArgument
	if errors.As(err, &invalid) {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid workflow query", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to query workflow stream", err)
	router.RespondWithError(c, reqErr.StatusCode, reqErr)
}

func isWorkflowTerminalStatus(status core.StatusType) bool {
	switch status {
	case core.StatusSuccess, core.StatusFailed, core.StatusTimedOut, core.StatusCanceled:
		return true
	default:
		return false
	}
}
