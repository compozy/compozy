package wfrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	workflowStreamDefaultPoll   = 500 * time.Millisecond
	workflowStreamMinPoll       = 250 * time.Millisecond
	workflowStreamMaxPoll       = 2000 * time.Millisecond
	workflowStreamHeartbeatFreq = 15 * time.Second
	workflowStreamQueryTimeout  = 5 * time.Second
)

type workflowStreamTunables struct {
	defaultPoll  time.Duration
	minPoll      time.Duration
	maxPoll      time.Duration
	heartbeat    time.Duration
	queryTimeout time.Duration
}

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

func parseWorkflowPollIntervalParam(c *gin.Context, tunables workflowStreamTunables) (time.Duration, bool) {
	interval, err := parseWorkflowPollInterval(c.Query("poll_ms"), tunables)
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

func parseWorkflowEventsParam(c *gin.Context) (map[string]struct{}, bool) {
	raw := strings.TrimSpace(c.Query("events"))
	if raw == "" {
		return nil, true
	}
	allowed := map[string]struct{}{
		wf.StreamEventWorkflowStart:  {},
		wf.StreamEventWorkflowStatus: {},
		wf.StreamEventComplete:       {},
		wf.StreamEventError:          {},
	}
	set := make(map[string]struct{}, len(allowed))
	for _, token := range strings.Split(raw, ",") {
		event := strings.TrimSpace(token)
		if event == "" {
			continue
		}
		if _, ok := allowed[event]; !ok {
			reqErr := router.NewRequestError(http.StatusBadRequest, "unknown event type", fmt.Errorf("%s", event))
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return nil, false
		}
		set[event] = struct{}{}
	}
	if len(set) == 0 {
		return nil, true
	}
	return set, true
}

func resolveWorkflowStreamContext(
	c *gin.Context,
) (workflowQueryClient, context.Context, logger.Logger, *monitoring.StreamingMetrics, bool) {
	state, ok := ensureWorkerReady(c)
	if !ok {
		return nil, nil, nil, nil, false
	}
	queryClient := resolveWorkflowQueryClient(state)
	if queryClient == nil {
		router.RespondProblemWithCode(
			c,
			http.StatusServiceUnavailable,
			router.ErrServiceUnavailableCode,
			"workflow query client unavailable",
		)
		return nil, nil, nil, nil, false
	}
	ctx := c.Request.Context()
	metrics := router.ResolveStreamingMetrics(c, state)
	return queryClient, ctx, logger.FromContext(ctx), metrics, true
}

// streamWorkflowExecution handles SSE streaming for workflow executions.
//
//	@Summary		Stream workflow execution events
//	@Description	Streams workflow progress over Server-Sent Events with Last-Event-ID resume support and configurable polling.
//	@Tags			executions
//	@Accept			*/*
//	@Produce		text/event-stream
//	@Param			exec_id		path		string											true	"Workflow execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Param			Last-Event-ID	header		string										false	"Resume the stream from the provided event id"		example("42")
//	@Param			poll_ms		query		int												false	"Polling interval (milliseconds). Default 500, min 250, max 2000."	example(500)
//	@Param			events		query		string											false	"Comma-separated list of event types to emit (default: all events)."	example("workflow_status,tool_call,llm_chunk,complete")
//	@Success		200			{string}	string											"SSE stream"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}			"Invalid request"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}			"Execution not found"
//	@Failure		503			{object}	router.Response{error=router.ErrorInfo}			"Worker unavailable"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}			"Internal server error"
//	@Router			/executions/workflows/{exec_id}/stream [get]
func streamWorkflowExecution(c *gin.Context) {
	execID, ok := parseWorkflowExecID(c)
	if !ok {
		return
	}
	tunables := resolveWorkflowStreamTunables(c.Request.Context())
	pollInterval, ok := parseWorkflowPollIntervalParam(c, tunables)
	if !ok {
		return
	}
	lastEventID, ok := parseLastEventIDHeader(c)
	if !ok {
		return
	}
	events, ok := parseWorkflowEventsParam(c)
	if !ok {
		return
	}
	queryClient, ctx, log, metrics, ok := resolveWorkflowStreamContext(c)
	if !ok {
		return
	}
	processWorkflowStream(ctx, c, execID, queryClient, pollInterval, lastEventID, events, tunables, log, metrics)
}

func prepareWorkflowStream(
	ctx context.Context,
	c *gin.Context,
	execID core.ID,
	client workflowQueryClient,
	queryTimeout time.Duration,
) (*wf.StreamState, core.StatusType, *router.SSEStream, bool) {
	snapshot, status, err := fetchWorkflowStreamState(ctx, client, execID.String(), queryTimeout)
	if err != nil {
		respondWorkflowQueryError(c, err)
		return nil, core.StatusPending, nil, false
	}
	stream := router.StartSSE(c.Writer)
	if stream == nil {
		router.RespondProblemWithCode(
			c,
			http.StatusInternalServerError,
			router.ErrInternalCode,
			"failed to initialize stream",
		)
		return nil, status, nil, false
	}
	return snapshot, status, stream, true
}

func initWorkflowTelemetry(
	ctx context.Context,
	execID core.ID,
	pollInterval time.Duration,
	lastEventID int64,
	status core.StatusType,
	metrics *monitoring.StreamingMetrics,
	log logger.Logger,
) (context.Context, router.StreamTelemetry, *router.StreamCloseInfo, func()) {
	closeInfo := &router.StreamCloseInfo{
		Reason:      router.StreamReasonInitializing,
		Status:      status,
		LastEventID: lastEventID,
	}
	telemetry := router.NewStreamTelemetry(ctx, monitoring.ExecutionKindWorkflow, execID, metrics, log)
	if telemetry != nil {
		telemetry.Connected(lastEventID, "Workflow stream connected", "poll_interval", pollInterval)
		return telemetry.Context(), telemetry, closeInfo, func() {
			telemetry.Close(closeInfo)
		}
	}
	started := time.Now()
	if log != nil {
		log.Info(
			"Workflow stream connected",
			"exec_id", execID,
			"last_event_id", lastEventID,
			"poll_interval", pollInterval,
		)
	}
	return ctx, nil, closeInfo, buildWorkflowFinalize(log, execID, started, closeInfo)
}

func buildWorkflowFinalize(
	log logger.Logger,
	execID core.ID,
	started time.Time,
	closeInfo *router.StreamCloseInfo,
) func() {
	return func() {
		if log == nil {
			return
		}
		duration := time.Since(started)
		info := closeInfo
		if info == nil {
			info = &router.StreamCloseInfo{Reason: "unknown"}
		}
		fields := []any{
			"exec_id", execID,
			"duration", duration,
			"last_event_id", info.LastEventID,
			"reason", info.Reason,
		}
		if info.Status != nil {
			fields = append(fields, "status", info.Status)
		}
		if info.Error != nil {
			log.Warn("workflow stream terminated", append(fields, "error", info.Error)...)
			return
		}
		log.Info("Workflow stream disconnected", fields...)
	}
}

func processWorkflowStream(
	ctx context.Context,
	c *gin.Context,
	execID core.ID,
	queryClient workflowQueryClient,
	pollInterval time.Duration,
	lastEventID int64,
	allowedEvents map[string]struct{},
	tunables workflowStreamTunables,
	log logger.Logger,
	metrics *monitoring.StreamingMetrics,
) {
	snapshot, status, stream, ok := prepareWorkflowStream(ctx, c, execID, queryClient, tunables.queryTimeout)
	if !ok {
		return
	}
	ctx, telemetry, closeInfo, finalize := initWorkflowTelemetry(
		ctx,
		execID,
		pollInterval,
		lastEventID,
		status,
		metrics,
		log,
	)
	defer finalize()
	workflowStreamLifecycle(
		ctx,
		stream,
		queryClient,
		pollInterval,
		lastEventID,
		snapshot,
		status,
		telemetry,
		closeInfo,
		log,
		execID,
		allowedEvents,
		tunables,
	)
}

func workflowStreamLifecycle(
	ctx context.Context,
	stream *router.SSEStream,
	queryClient workflowQueryClient,
	pollInterval time.Duration,
	lastEventID int64,
	snapshot *wf.StreamState,
	status core.StatusType,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	log logger.Logger,
	execID core.ID,
	allowedEvents map[string]struct{},
	tunables workflowStreamTunables,
) {
	updatedID, cont, snapErr := writeInitialSnapshot(
		stream,
		telemetry,
		log,
		execID,
		snapshot,
		status,
		lastEventID,
		allowedEvents,
	)
	if handleWorkflowSnapshotResult(closeInfo, snapErr, cont, updatedID) {
		return
	}
	finalStatus, loopErr := runWorkflowStreamLoop(
		ctx,
		stream,
		execID,
		queryClient,
		pollInterval,
		&updatedID,
		telemetry,
		allowedEvents,
		tunables,
	)
	finalizeWorkflowResult(closeInfo, finalStatus, loopErr, updatedID)
}

func handleWorkflowSnapshotResult(
	closeInfo *router.StreamCloseInfo,
	snapErr error,
	cont bool,
	updatedID int64,
) bool {
	if closeInfo != nil {
		closeInfo.LastEventID = updatedID
	}
	if snapErr != nil {
		if closeInfo != nil {
			closeInfo.Reason = "initial_snapshot_failed"
			closeInfo.Error = snapErr
		}
		return true
	}
	if !cont {
		if closeInfo != nil {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return true
	}
	return false
}

func finalizeWorkflowResult(
	closeInfo *router.StreamCloseInfo,
	finalStatus core.StatusType,
	loopErr error,
	updatedID int64,
) {
	if closeInfo != nil {
		closeInfo.LastEventID = updatedID
		closeInfo.Status = finalStatus
	}
	if loopErr != nil {
		if errors.Is(loopErr, context.Canceled) {
			if closeInfo != nil {
				closeInfo.Reason = "context_canceled"
				closeInfo.Error = nil
			}
			return
		}
		if closeInfo != nil {
			closeInfo.Reason = "stream_error"
			closeInfo.Error = loopErr
		}
		return
	}
	if closeInfo != nil {
		closeInfo.Reason = "completed"
		closeInfo.Error = nil
	}
}

func writeInitialSnapshot(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	log logger.Logger,
	execID core.ID,
	snapshot *wf.StreamState,
	status core.StatusType,
	lastEventID int64,
	allowedEvents map[string]struct{},
) (int64, bool, error) {
	updatedID, err := emitWorkflowEvents(stream, snapshot, lastEventID, telemetry, allowedEvents)
	if err != nil {
		if log != nil {
			log.Warn("workflow stream write failed", "exec_id", execID, "error", err)
		}
		return lastEventID, false, err
	}
	if isWorkflowTerminalStatus(status) {
		return updatedID, false, nil
	}
	return updatedID, true, nil
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
	timeout time.Duration,
) (*wf.StreamState, core.StatusType, error) {
	queryTimeout := timeout
	if queryTimeout <= 0 {
		queryTimeout = workflowStreamQueryTimeout
	}
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
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
	telemetry router.StreamTelemetry,
	allowedEvents map[string]struct{},
	tunables workflowStreamTunables,
) (core.StatusType, error) {
	heartbeatInterval := tunables.heartbeat
	if heartbeatInterval <= 0 {
		heartbeatInterval = workflowStreamHeartbeatFreq
	}
	heartbeatTicker := time.NewTicker(heartbeatInterval)
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
			if telemetry != nil {
				telemetry.RecordHeartbeat()
			}
		case <-pollTicker.C:
			currentStatus, err := processWorkflowPoll(
				ctx,
				stream,
				execID,
				client,
				lastEventID,
				telemetry,
				allowedEvents,
				tunables,
			)
			if err != nil {
				return status, err
			}
			status = currentStatus
			if isWorkflowTerminalStatus(currentStatus) {
				return status, nil
			}
		}
	}
}

func processWorkflowPoll(
	ctx context.Context,
	stream *router.SSEStream,
	execID core.ID,
	client workflowQueryClient,
	lastEventID *int64,
	telemetry router.StreamTelemetry,
	allowedEvents map[string]struct{},
	tunables workflowStreamTunables,
) (core.StatusType, error) {
	snapshot, currentStatus, err := fetchWorkflowStreamState(
		ctx,
		client,
		execID.String(),
		tunables.queryTimeout,
	)
	if err != nil {
		return currentStatus, err
	}
	updatedID, writeErr := emitWorkflowEvents(stream, snapshot, *lastEventID, telemetry, allowedEvents)
	if writeErr != nil {
		return currentStatus, writeErr
	}
	*lastEventID = updatedID
	return currentStatus, nil
}

func emitWorkflowEvents(
	stream *router.SSEStream,
	state *wf.StreamState,
	lastID int64,
	telemetry router.StreamTelemetry,
	allowedEvents map[string]struct{},
) (int64, error) {
	if stream == nil || state == nil {
		return lastID, nil
	}
	for _, event := range state.Events {
		if event.ID <= lastID {
			continue
		}
		if !shouldEmitWorkflowEvent(allowedEvents, event.Type) {
			continue
		}
		if err := stream.WriteEvent(event.ID, event.Type, event.Data); err != nil {
			return lastID, err
		}
		if telemetry != nil {
			telemetry.RecordEvent(event.Type, true)
		}
		lastID = event.ID
	}
	return lastID, nil
}

func shouldEmitWorkflowEvent(allowed map[string]struct{}, event string) bool {
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[event]
	return ok
}

func parseWorkflowPollInterval(raw string, tunables workflowStreamTunables) (time.Duration, error) {
	if raw == "" {
		return tunables.defaultPoll, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid poll_ms: %w", err)
	}
	minMillis := int(tunables.minPoll / time.Millisecond)
	if ms < minMillis {
		return 0, fmt.Errorf("poll_ms must be >= %d", minMillis)
	}
	maxMillis := int(tunables.maxPoll / time.Millisecond)
	if ms > maxMillis {
		return 0, fmt.Errorf("poll_ms must be <= %d", maxMillis)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func resolveWorkflowStreamTunables(ctx context.Context) workflowStreamTunables {
	values := workflowStreamTunables{
		defaultPoll:  workflowStreamDefaultPoll,
		minPoll:      workflowStreamMinPoll,
		maxPoll:      workflowStreamMaxPoll,
		heartbeat:    workflowStreamHeartbeatFreq,
		queryTimeout: workflowStreamQueryTimeout,
	}
	cfg := config.FromContext(ctx)
	if cfg != nil {
		if poll := cfg.Stream.Workflow.DefaultPoll; poll > 0 {
			values.defaultPoll = poll
		}
		if minPoll := cfg.Stream.Workflow.MinPoll; minPoll > 0 {
			values.minPoll = minPoll
		}
		if maxPoll := cfg.Stream.Workflow.MaxPoll; maxPoll > 0 {
			values.maxPoll = maxPoll
		}
		if hb := cfg.Stream.Workflow.HeartbeatFrequency; hb > 0 {
			values.heartbeat = hb
		}
		if timeout := cfg.Stream.Workflow.QueryTimeout; timeout > 0 {
			values.queryTimeout = timeout
		}
	}
	if values.minPoll <= 0 {
		values.minPoll = workflowStreamMinPoll
	}
	if values.maxPoll <= 0 {
		values.maxPoll = workflowStreamMaxPoll
	}
	if values.defaultPoll <= 0 {
		values.defaultPoll = workflowStreamDefaultPoll
	}
	if values.heartbeat <= 0 {
		values.heartbeat = workflowStreamHeartbeatFreq
	}
	if values.queryTimeout <= 0 {
		values.queryTimeout = workflowStreamQueryTimeout
	}
	if values.minPoll > values.maxPoll {
		values.minPoll = values.maxPoll
	}
	if values.defaultPoll < values.minPoll {
		values.defaultPoll = values.minPoll
	}
	if values.defaultPoll > values.maxPoll {
		values.defaultPoll = values.maxPoll
	}
	return values
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
