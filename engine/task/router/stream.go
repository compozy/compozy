package tkrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/pubsub"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/resources"
	taskdomain "github.com/compozy/compozy/engine/task"
	taskuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	taskStreamDefaultPoll   = 500 * time.Millisecond
	taskStreamMinPoll       = 250 * time.Millisecond
	taskStreamMaxPoll       = 2000 * time.Millisecond
	taskStreamHeartbeatFreq = 15 * time.Second
	taskStatusEvent         = "task_status"
	completeEvent           = "complete"
	errorEvent              = "error"
	llmChunkEvent           = "llm_chunk"
)

type taskStreamMode int

const (
	streamModeStructured taskStreamMode = iota
	streamModeText
)

func (m taskStreamMode) String() string {
	switch m {
	case streamModeStructured:
		return "structured"
	case streamModeText:
		return "text"
	default:
		return "unknown"
	}
}

type taskStreamConfig struct {
	execID       core.ID
	repo         taskdomain.Repository
	pubsub       pubsub.Provider
	initial      *taskdomain.State
	pollInterval time.Duration
	lastEventID  int64
	mode         taskStreamMode
	log          logger.Logger
	metrics      *monitoring.StreamingMetrics
}

// streamTaskExecution streams Server-Sent Events for task executions.
//
//	@Summary		Stream task execution events
//	@Description	Streams task execution updates over Server-Sent Events, emitting structured JSON or llm_chunk text depending on the task output schema.
//	@Tags			executions
//	@Accept			*/*
//	@Produce		text/event-stream
//	@Param			exec_id	path		string											true	"Task execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Param			Last-Event-ID	header		string											false	"Resume the stream from the provided event id"	example("42")
//	@Param			poll_ms			query		int												false	"Polling interval (milliseconds). Default 500, min 250, max 2000."	example(500)
//	@Param			events			query		string											false	"Comma-separated list of event types to emit (default: all events)."	example("task_status,llm_chunk,complete")
//	@Success		200				{string}	string											"SSE stream"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}			"Invalid request"
//	@Failure		404				{object}	router.Response{error=router.ErrorInfo}			"Execution not found"
//	@Failure		503				{object}	router.Response{error=router.ErrorInfo}			"Pub/Sub provider unavailable"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}			"Internal server error"
//	@Router			/executions/tasks/{exec_id}/stream [get]
func streamTaskExecution(c *gin.Context) {
	execID := router.GetTaskExecID(c)
	if execID == "" {
		return
	}
	ctx := c.Request.Context()
	cfg, ok := prepareTaskStreamConfig(ctx, c, execID)
	if !ok {
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
	runTaskStream(ctx, cfg, stream)
}

func initTaskTelemetry(
	ctx context.Context,
	cfg *taskStreamConfig,
) (context.Context, router.StreamTelemetry, *router.StreamCloseInfo, func()) {
	closeInfo := &router.StreamCloseInfo{
		Reason:      router.StreamReasonInitializing,
		Status:      cfg.initial.Status,
		LastEventID: cfg.lastEventID,
		ExtraFields: []any{"mode", cfg.mode.String()},
	}
	telemetry := router.NewStreamTelemetry(ctx, monitoring.ExecutionKindTask, cfg.execID, cfg.metrics, cfg.log)
	if telemetry != nil {
		telemetry.Connected(cfg.lastEventID, "Task stream connected", "mode", cfg.mode.String())
		return telemetry.Context(), telemetry, closeInfo, func() {
			telemetry.Close(closeInfo)
		}
	}
	started := time.Now()
	if cfg.log != nil {
		cfg.log.Info(
			"Task stream connected",
			"exec_id", cfg.execID,
			"mode", cfg.mode.String(),
			"last_event_id", cfg.lastEventID,
		)
	}
	finalize := func() {
		if cfg.log == nil {
			return
		}
		duration := time.Since(started)
		fields := []any{
			"exec_id", cfg.execID,
			"duration", duration,
			"last_event_id", closeInfo.LastEventID,
			"reason", closeInfo.Reason,
		}
		if closeInfo.Status != nil {
			fields = append(fields, "status", closeInfo.Status)
		}
		if len(closeInfo.ExtraFields) > 0 {
			fields = append(fields, closeInfo.ExtraFields...)
		}
		if closeInfo.Error != nil {
			cfg.log.Warn("task stream terminated", append(fields, "error", closeInfo.Error)...)
			return
		}
		cfg.log.Info("Task stream disconnected", fields...)
	}
	return ctx, nil, closeInfo, finalize
}

func prepareTaskStreamConfig(ctx context.Context, c *gin.Context, execID core.ID) (*taskStreamConfig, bool) {
	state := router.GetAppState(c)
	if state == nil {
		return nil, false
	}
	repo := router.ResolveTaskRepository(c, state)
	if repo == nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "task repository unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	resourceStore, ok := router.GetResourceStore(c)
	if !ok {
		return nil, false
	}
	execState, ok := fetchTaskExecutionState(ctx, c, repo, execID)
	if !ok {
		return nil, false
	}
	pollInterval, ok := parseTaskPollIntervalParam(c)
	if !ok {
		return nil, false
	}
	lastEventID, ok := parseTaskLastEventID(c)
	if !ok {
		return nil, false
	}
	mode, err := determineTaskStreamMode(ctx, state, resourceStore, execState)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to resolve task stream mode", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	pubsubProvider, ok := resolveTaskPubSubDependency(c, state, mode)
	if !ok {
		return nil, false
	}
	log := logger.FromContext(ctx)
	metrics := router.ResolveStreamingMetrics(c, state)
	return &taskStreamConfig{
		execID:       execID,
		repo:         repo,
		pubsub:       pubsubProvider,
		initial:      execState,
		pollInterval: pollInterval,
		lastEventID:  lastEventID,
		mode:         mode,
		log:          log,
		metrics:      metrics,
	}, true
}

func fetchTaskExecutionState(
	ctx context.Context,
	c *gin.Context,
	repo taskdomain.Repository,
	execID core.ID,
) (*taskdomain.State, bool) {
	state, err := repo.GetState(ctx, execID)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			reqErr := router.NewRequestError(http.StatusNotFound, "execution not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return nil, false
		}
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to load execution", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return state, true
}

func parseTaskPollIntervalParam(c *gin.Context) (time.Duration, bool) {
	interval, err := parseTaskPollInterval(c.Query("poll_ms"))
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid poll interval", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return interval, true
}

func parseTaskPollInterval(raw string) (time.Duration, error) {
	if raw == "" {
		return taskStreamDefaultPoll, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid poll_ms: %w", err)
	}
	if ms < int(taskStreamMinPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be >= %d", taskStreamMinPoll/time.Millisecond)
	}
	if ms > int(taskStreamMaxPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be <= %d", taskStreamMaxPoll/time.Millisecond)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func parseTaskLastEventID(c *gin.Context) (int64, bool) {
	lastID, _, err := router.LastEventID(c.Request)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid Last-Event-ID header", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return lastID, true
}

func determineTaskStreamMode(
	ctx context.Context,
	state *appstate.State,
	store resources.ResourceStore,
	execState *taskdomain.State,
) (taskStreamMode, error) {
	hasStructured, err := taskHasStructuredOutput(ctx, state, store, execState)
	if err != nil {
		return streamModeStructured, err
	}
	if hasStructured {
		return streamModeStructured, nil
	}
	return streamModeText, nil
}

func taskHasStructuredOutput(
	ctx context.Context,
	state *appstate.State,
	store resources.ResourceStore,
	execState *taskdomain.State,
) (bool, error) {
	if execState == nil {
		return false, nil
	}
	taskID := strings.TrimSpace(execState.TaskID)
	if taskID == "" {
		return false, nil
	}
	cfg, err := loadTaskConfig(ctx, state, store, taskID)
	if err != nil {
		return false, err
	}
	return cfg.OutputSchema != nil, nil
}

func loadTaskConfig(
	ctx context.Context,
	state *appstate.State,
	store resources.ResourceStore,
	taskID string,
) (*taskdomain.Config, error) {
	projectName := resolveProjectName(ctx, state)
	if projectName == "" {
		return nil, fmt.Errorf("project name not available in context")
	}
	getUC := taskuc.NewGet(store)
	out, err := getUC.Execute(ctx, &taskuc.GetInput{Project: projectName, ID: taskID})
	if err != nil {
		return nil, err
	}
	cfg := &taskdomain.Config{}
	if err := cfg.FromMap(out.Task); err != nil {
		return nil, fmt.Errorf("decode task config: %w", err)
	}
	return cfg, nil
}

func resolveProjectName(ctx context.Context, state *appstate.State) string {
	if project, err := core.GetProjectName(ctx); err == nil && project != "" {
		return project
	}
	if state != nil && state.ProjectConfig != nil {
		return state.ProjectConfig.Name
	}
	return ""
}

func resolveTaskPubSubDependency(c *gin.Context, state *appstate.State, mode taskStreamMode) (pubsub.Provider, bool) {
	if mode != streamModeText {
		return nil, true
	}
	provider := router.ResolvePubSubProvider(c, state)
	if provider == nil {
		return nil, false
	}
	return provider, true
}

func runTaskStream(ctx context.Context, cfg *taskStreamConfig, stream *router.SSEStream) {
	ctx, telemetry, closeInfo, finalize := initTaskTelemetry(ctx, cfg)
	defer finalize()
	switch cfg.mode {
	case streamModeStructured:
		runTaskStructuredStream(ctx, cfg, stream, telemetry, closeInfo)
	default:
		runTaskTextStream(ctx, cfg, stream, telemetry, closeInfo)
	}
	if closeInfo.Reason == router.StreamReasonInitializing {
		closeInfo.Reason = "completed"
	}
}

func runTaskStructuredStream(
	ctx context.Context,
	cfg *taskStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
) {
	nextID, cont := emitTaskInitialEvents(stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(taskStreamHeartbeatFreq)
	defer ticker.Stop()
	defer heartbeat.Stop()
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	for {
		select {
		case <-ctx.Done():
			if closeInfo != nil {
				closeInfo.Reason = "context_canceled"
				closeInfo.Error = nil
			}
			logTaskCancellation(cfg)
			return
		case <-heartbeat.C:
			if err := stream.WriteHeartbeat(); err != nil {
				logTaskStreamError(cfg, "heartbeat", err, closeInfo)
				return
			}
			if telemetry != nil {
				telemetry.RecordHeartbeat()
			}
		case <-ticker.C:
			updatedID, ok := handleTaskPoll(ctx, stream, cfg, telemetry, closeInfo, nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		}
	}
}

func runTaskTextStream(
	ctx context.Context,
	cfg *taskStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
) {
	subscription, err := cfg.pubsub.Subscribe(ctx, redisTokenChannel(cfg.execID))
	if err != nil {
		logTaskStreamError(cfg, "subscribe", err, closeInfo)
		return
	}
	defer subscription.Close()
	nextID, cont := emitTaskInitialEvents(stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	taskTextStreamLoop(ctx, cfg, telemetry, closeInfo, stream, subscription.Messages(), nextID)
}

func taskTextStreamLoop(
	ctx context.Context,
	cfg *taskStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	stream *router.SSEStream,
	messages <-chan pubsub.Message,
	startID int64,
) {
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(taskStreamHeartbeatFreq)
	defer ticker.Stop()
	defer heartbeat.Stop()
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	nextID := startID
	for {
		select {
		case <-ctx.Done():
			if closeInfo != nil {
				closeInfo.Reason = "context_canceled"
				closeInfo.Error = nil
			}
			logTaskCancellation(cfg)
			return
		case <-heartbeat.C:
			if err := stream.WriteHeartbeat(); err != nil {
				logTaskStreamError(cfg, "heartbeat", err, closeInfo)
				return
			}
			if telemetry != nil {
				telemetry.RecordHeartbeat()
			}
		case msg, ok := <-messages:
			updatedID, ok := handleTaskChunk(stream, cfg, telemetry, closeInfo, nextID, msg, ok)
			if !ok {
				return
			}
			nextID = updatedID
		case <-ticker.C:
			updatedID, ok := handleTaskPoll(ctx, stream, cfg, telemetry, closeInfo, nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		}
	}
}

func emitTaskInitialEvents(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	cfg *taskStreamConfig,
	state *taskdomain.State,
	lastID int64,
	closeInfo *router.StreamCloseInfo,
) (int64, bool) {
	nextID, err := emitTaskStatus(stream, telemetry, state, lastID)
	if err != nil {
		logTaskStreamError(cfg, "status", err, closeInfo)
		return lastID, false
	}
	if closeInfo != nil {
		closeInfo.LastEventID = nextID
		closeInfo.Status = state.Status
	}
	if isTerminalStatus(state.Status) {
		updatedID, termErr := emitTaskTerminal(stream, telemetry, state, nextID)
		if termErr != nil {
			logTaskStreamError(cfg, "terminal", termErr, closeInfo)
			return nextID, false
		}
		if closeInfo != nil {
			closeInfo.LastEventID = updatedID
			closeInfo.Status = state.Status
			closeInfo.Reason = router.StreamReasonTerminal
		}
		logTaskCompletion(cfg, updatedID, state.Status)
		return updatedID, false
	}
	return nextID, true
}

func emitTaskStatus(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	state *taskdomain.State,
	lastID int64,
) (int64, error) {
	payload := taskStatusPayload{
		Status:    state.Status,
		UpdatedAt: state.UpdatedAt.UTC(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return lastID, err
	}
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, taskStatusEvent, data); err != nil {
		return lastID, err
	}
	if telemetry != nil {
		telemetry.RecordEvent(taskStatusEvent, true)
	}
	return nextID, nil
}

func emitTaskTerminal(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	state *taskdomain.State,
	lastID int64,
) (int64, error) {
	payload := taskTerminalPayload{
		Status:    state.Status,
		Result:    state.Output,
		Error:     state.Error,
		Usage:     router.NewUsageSummary(state.Usage),
		UpdatedAt: state.UpdatedAt.UTC(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return lastID, err
	}
	eventType := completeEvent
	if state.Status != core.StatusSuccess {
		eventType = errorEvent
	}
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, eventType, data); err != nil {
		return lastID, err
	}
	if telemetry != nil {
		telemetry.RecordEvent(eventType, true)
	}
	return nextID, nil
}

func logTaskStreamError(cfg *taskStreamConfig, phase string, err error, closeInfo *router.StreamCloseInfo) {
	if cfg.log == nil {
		return
	}
	cfg.log.Warn(
		"task stream terminated with error",
		"exec_id", cfg.execID,
		"phase", phase,
		"error", err,
	)
	if closeInfo != nil && err != nil && closeInfo.Error == nil {
		closeInfo.Reason = fmt.Sprintf("%s_error", phase)
		closeInfo.Error = err
	}
}

func logTaskCompletion(cfg *taskStreamConfig, lastID int64, status core.StatusType) {
	if cfg.log == nil {
		return
	}
	cfg.log.Info(
		"Task stream disconnected",
		"exec_id", cfg.execID,
		"last_event_id", lastID,
		"status", status,
	)
}

func logTaskCancellation(cfg *taskStreamConfig) {
	if cfg.log == nil {
		return
	}
	cfg.log.Info("Task stream canceled", "exec_id", cfg.execID)
}

func isTerminalStatus(status core.StatusType) bool {
	switch status {
	case core.StatusSuccess, core.StatusFailed, core.StatusTimedOut, core.StatusCanceled:
		return true
	default:
		return false
	}
}

func redisTokenChannel(execID core.ID) string {
	return fmt.Sprintf("stream:tokens:%s", execID.String())
}

func handleTaskChunk(
	stream *router.SSEStream,
	cfg *taskStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
	msg pubsub.Message,
	ok bool,
) (int64, bool) {
	if !ok {
		logTaskStreamError(cfg, "pubsub", errors.New("message channel closed"), closeInfo)
		return lastID, false
	}
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, llmChunkEvent, msg.Payload); err != nil {
		logTaskStreamError(cfg, "chunk", err, closeInfo)
		return lastID, false
	}
	if telemetry != nil {
		telemetry.RecordEvent(llmChunkEvent, true)
	}
	if closeInfo != nil {
		closeInfo.LastEventID = nextID
	}
	return nextID, true
}

func handleTaskPoll(
	ctx context.Context,
	stream *router.SSEStream,
	cfg *taskStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
	lastStatus *core.StatusType,
	lastUpdated *time.Time,
) (int64, bool) {
	state, err := cfg.repo.GetState(ctx, cfg.execID)
	if err != nil {
		logTaskStreamError(cfg, "poll", err, closeInfo)
		return lastID, false
	}
	updatedID := lastID
	if state.Status != *lastStatus || state.UpdatedAt.After(*lastUpdated) {
		updatedID, err = emitTaskStatus(stream, telemetry, state, lastID)
		if err != nil {
			logTaskStreamError(cfg, "status", err, closeInfo)
			return lastID, false
		}
		*lastStatus = state.Status
		*lastUpdated = state.UpdatedAt
		if closeInfo != nil {
			closeInfo.LastEventID = updatedID
			closeInfo.Status = state.Status
		}
	}
	if isTerminalStatus(state.Status) {
		updatedID, err = emitTaskTerminal(stream, telemetry, state, updatedID)
		if err != nil {
			logTaskStreamError(cfg, "terminal", err, closeInfo)
			return lastID, false
		}
		if closeInfo != nil {
			closeInfo.LastEventID = updatedID
			closeInfo.Status = state.Status
			closeInfo.Reason = router.StreamReasonTerminal
		}
		logTaskCompletion(cfg, updatedID, state.Status)
		return updatedID, false
	}
	return updatedID, true
}

type taskStatusPayload struct {
	Status    core.StatusType `json:"status"`
	UpdatedAt time.Time       `json:"ts"`
}

type taskTerminalPayload struct {
	Status    core.StatusType      `json:"status"`
	Result    *core.Output         `json:"result,omitempty"`
	Error     *core.Error          `json:"error,omitempty"`
	Usage     *router.UsageSummary `json:"usage,omitempty"`
	UpdatedAt time.Time            `json:"ts"`
}
