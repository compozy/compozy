package agentrouter

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

	agentcfg "github.com/compozy/compozy/engine/agent"
	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/pubsub"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	agentStreamDefaultPoll   = 500 * time.Millisecond
	agentStreamMinPoll       = 250 * time.Millisecond
	agentStreamMaxPoll       = 2000 * time.Millisecond
	agentStreamHeartbeatFreq = 15 * time.Second
	agentStatusEvent         = "agent_status"
	completeEvent            = "complete"
	errorEvent               = "error"
	llmChunkEvent            = "llm_chunk"
	promptActionID           = "__prompt__"
)

type streamMode int

const (
	streamModeStructured streamMode = iota
	streamModeText
)

func (m streamMode) String() string {
	switch m {
	case streamModeStructured:
		return "structured"
	case streamModeText:
		return "text"
	default:
		return "unknown"
	}
}

type agentStreamConfig struct {
	execID       core.ID
	repo         task.Repository
	pubsub       pubsub.Provider
	initial      *task.State
	pollInterval time.Duration
	lastEventID  int64
	mode         streamMode
	log          logger.Logger
	metrics      *monitoring.StreamingMetrics
}

// streamAgentExecution streams Server-Sent Events for agent executions.
//
//	@Summary		Stream agent execution events
//	@Description	Streams agent execution updates over Server-Sent Events, emitting structured JSON or llm_chunk text depending on the output schema.
//	@Tags			executions
//	@Accept			*/*
//	@Produce		text/event-stream
//	@Param			agent_exec_id	path		string											true	"Agent execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Param			Last-Event-ID	header		string											false	"Resume the stream from the provided event id"	example("42")
//	@Param			poll_ms			query		int												false	"Polling interval (milliseconds). Default 500, min 250, max 2000."	example(500)
//	@Param			events			query		string											false	"Comma-separated list of event types to emit (default: all events)."	example("agent_status,llm_chunk,complete")
//	@Success		200				{string}	string											"SSE stream"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}			"Invalid request"
//	@Failure		404				{object}	router.Response{error=router.ErrorInfo}			"Execution not found"
//	@Failure		503				{object}	router.Response{error=router.ErrorInfo}			"Pub/Sub provider unavailable"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}			"Internal server error"
//	@Router			/executions/agents/{agent_exec_id}/stream [get]
func streamAgentExecution(c *gin.Context) {
	execID := router.GetAgentExecID(c)
	if execID == "" {
		return
	}
	ctx := c.Request.Context()
	cfg, ok := prepareAgentStreamConfig(ctx, c, execID)
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
	runAgentStream(ctx, cfg, stream)
}

func initAgentTelemetry(
	ctx context.Context,
	cfg *agentStreamConfig,
) (context.Context, router.StreamTelemetry, *router.StreamCloseInfo, func()) {
	closeInfo := &router.StreamCloseInfo{
		Reason:      router.StreamReasonInitializing,
		Status:      cfg.initial.Status,
		LastEventID: cfg.lastEventID,
		ExtraFields: []any{"mode", cfg.mode.String()},
	}
	telemetry := router.NewStreamTelemetry(ctx, monitoring.ExecutionKindAgent, cfg.execID, cfg.metrics, cfg.log)
	if telemetry != nil {
		telemetry.Connected(cfg.lastEventID, "Agent stream connected", "mode", cfg.mode.String())
		return telemetry.Context(), telemetry, closeInfo, func() {
			telemetry.Close(closeInfo)
		}
	}
	started := time.Now()
	if cfg.log != nil {
		cfg.log.Info(
			"Agent stream connected",
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
			cfg.log.Warn("agent stream terminated", append(fields, "error", closeInfo.Error)...)
			return
		}
		cfg.log.Info("Agent stream disconnected", fields...)
	}
	return ctx, nil, closeInfo, finalize
}

func prepareAgentStreamConfig(ctx context.Context, c *gin.Context, execID core.ID) (*agentStreamConfig, bool) {
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
	taskState, ok := fetchAgentTaskState(ctx, c, repo, execID)
	if !ok {
		return nil, false
	}
	pollInterval, ok := parseAgentPollIntervalParam(c)
	if !ok {
		return nil, false
	}
	lastEventID, ok := parseAgentLastEventID(c)
	if !ok {
		return nil, false
	}
	mode, err := determineAgentStreamMode(ctx, state, resourceStore, taskState)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to resolve agent stream mode", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	pubsubProvider, ok := resolveAgentPubSubDependency(c, state, mode)
	if !ok {
		return nil, false
	}
	log := logger.FromContext(ctx)
	metrics := router.ResolveStreamingMetrics(c, state)
	return &agentStreamConfig{
		execID:       execID,
		repo:         repo,
		pubsub:       pubsubProvider,
		initial:      taskState,
		pollInterval: pollInterval,
		lastEventID:  lastEventID,
		mode:         mode,
		log:          log,
		metrics:      metrics,
	}, true
}

func fetchAgentTaskState(
	ctx context.Context,
	c *gin.Context,
	repo task.Repository,
	execID core.ID,
) (*task.State, bool) {
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

func parseAgentPollIntervalParam(c *gin.Context) (time.Duration, bool) {
	interval, err := parseAgentPollInterval(c.Query("poll_ms"))
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid poll interval", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return interval, true
}

func parseAgentPollInterval(raw string) (time.Duration, error) {
	if raw == "" {
		return agentStreamDefaultPoll, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid poll_ms: %w", err)
	}
	if ms < int(agentStreamMinPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be >= %d", agentStreamMinPoll/time.Millisecond)
	}
	if ms > int(agentStreamMaxPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be <= %d", agentStreamMaxPoll/time.Millisecond)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func parseAgentLastEventID(c *gin.Context) (int64, bool) {
	lastID, _, err := router.LastEventID(c.Request)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid Last-Event-ID header", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return lastID, true
}

func determineAgentStreamMode(
	ctx context.Context,
	state *appstate.State,
	store resources.ResourceStore,
	execState *task.State,
) (streamMode, error) {
	hasStructured, err := agentHasStructuredOutput(ctx, state, store, execState)
	if err != nil {
		return streamModeStructured, err
	}
	if hasStructured {
		return streamModeStructured, nil
	}
	return streamModeText, nil
}

func agentHasStructuredOutput(
	ctx context.Context,
	state *appstate.State,
	store resources.ResourceStore,
	execState *task.State,
) (bool, error) {
	if execState == nil {
		return false, nil
	}
	if execState.AgentID == nil || execState.ActionID == nil {
		return false, nil
	}
	actionID := strings.TrimSpace(*execState.ActionID)
	if actionID == "" || actionID == promptActionID {
		return false, nil
	}
	agentID := strings.TrimSpace(*execState.AgentID)
	if agentID == "" {
		return false, nil
	}
	cfg, err := loadAgentConfig(ctx, state, store, agentID)
	if err != nil {
		return false, err
	}
	action, err := agentcfg.FindActionConfig(cfg.Actions, actionID)
	if err != nil {
		return false, err
	}
	return action.ShouldUseJSONOutput(), nil
}

func loadAgentConfig(
	ctx context.Context,
	state *appstate.State,
	store resources.ResourceStore,
	agentID string,
) (*agentcfg.Config, error) {
	projectName := resolveProjectName(ctx, state)
	if projectName == "" {
		return nil, fmt.Errorf("project name not available in context")
	}
	getUC := agentuc.NewGet(store)
	out, err := getUC.Execute(ctx, &agentuc.GetInput{Project: projectName, ID: agentID})
	if err != nil {
		return nil, err
	}
	cfg := &agentcfg.Config{}
	if err := cfg.FromMap(out.Agent); err != nil {
		return nil, fmt.Errorf("decode agent config: %w", err)
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

func resolveAgentPubSubDependency(
	c *gin.Context,
	state *appstate.State,
	mode streamMode,
) (pubsub.Provider, bool) {
	if mode != streamModeText {
		return nil, true
	}
	provider := router.ResolvePubSubProvider(c, state)
	if provider == nil {
		return nil, false
	}
	return provider, true
}

func runAgentStream(ctx context.Context, cfg *agentStreamConfig, stream *router.SSEStream) {
	ctx, telemetry, closeInfo, finalize := initAgentTelemetry(ctx, cfg)
	defer finalize()
	switch cfg.mode {
	case streamModeStructured:
		runAgentStructuredStream(ctx, cfg, stream, telemetry, closeInfo)
	default:
		runAgentTextStream(ctx, cfg, stream, telemetry, closeInfo)
	}
	if closeInfo.Reason == router.StreamReasonInitializing {
		closeInfo.Reason = "completed"
	}
}

func runAgentStructuredStream(
	ctx context.Context,
	cfg *agentStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
) {
	nextID, cont := emitAgentInitialEvents(stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(agentStreamHeartbeatFreq)
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
			logAgentCancellation(cfg)
			return
		case <-heartbeat.C:
			if err := stream.WriteHeartbeat(); err != nil {
				logAgentStreamError(cfg, "heartbeat", err, closeInfo)
				return
			}
			if telemetry != nil {
				telemetry.RecordHeartbeat()
			}
		case <-ticker.C:
			updatedID, ok := handleAgentPoll(ctx, stream, cfg, telemetry, closeInfo, nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		}
	}
}

func runAgentTextStream(
	ctx context.Context,
	cfg *agentStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
) {
	subscription, err := cfg.pubsub.Subscribe(ctx, redisTokenChannel(cfg.execID))
	if err != nil {
		logAgentStreamError(cfg, "subscribe", err, closeInfo)
		return
	}
	defer subscription.Close()
	nextID, cont := emitAgentInitialEvents(stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	agentTextStreamLoop(ctx, cfg, telemetry, closeInfo, stream, subscription.Messages(), nextID)
}

func agentTextStreamLoop(
	ctx context.Context,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	stream *router.SSEStream,
	messages <-chan pubsub.Message,
	startID int64,
) {
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(agentStreamHeartbeatFreq)
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
			logAgentCancellation(cfg)
			return
		case <-heartbeat.C:
			if err := stream.WriteHeartbeat(); err != nil {
				logAgentStreamError(cfg, "heartbeat", err, closeInfo)
				return
			}
			if telemetry != nil {
				telemetry.RecordHeartbeat()
			}
		case msg, ok := <-messages:
			updatedID, ok := handleAgentChunk(stream, cfg, telemetry, closeInfo, nextID, msg, ok)
			if !ok {
				return
			}
			nextID = updatedID
		case <-ticker.C:
			updatedID, ok := handleAgentPoll(ctx, stream, cfg, telemetry, closeInfo, nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		}
	}
}

func emitAgentInitialEvents(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	cfg *agentStreamConfig,
	state *task.State,
	lastID int64,
	closeInfo *router.StreamCloseInfo,
) (int64, bool) {
	nextID, err := emitAgentStatus(stream, telemetry, state, lastID)
	if err != nil {
		logAgentStreamError(cfg, "status", err, closeInfo)
		return lastID, false
	}
	if closeInfo != nil {
		closeInfo.LastEventID = nextID
		closeInfo.Status = state.Status
	}
	if isTerminalStatus(state.Status) {
		updatedID, termErr := emitAgentTerminal(stream, telemetry, state, nextID)
		if termErr != nil {
			logAgentStreamError(cfg, "terminal", termErr, closeInfo)
			return nextID, false
		}
		if closeInfo != nil {
			closeInfo.LastEventID = updatedID
			closeInfo.Status = state.Status
			closeInfo.Reason = router.StreamReasonTerminal
		}
		logAgentCompletion(cfg, updatedID, state.Status)
		return updatedID, false
	}
	return nextID, true
}

func emitAgentStatus(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	state *task.State,
	lastID int64,
) (int64, error) {
	payload := agentStatusPayload{
		Status:    state.Status,
		UpdatedAt: state.UpdatedAt.UTC(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return lastID, err
	}
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, agentStatusEvent, data); err != nil {
		return lastID, err
	}
	if telemetry != nil {
		telemetry.RecordEvent(agentStatusEvent, true)
	}
	return nextID, nil
}

func emitAgentTerminal(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	state *task.State,
	lastID int64,
) (int64, error) {
	payload := agentTerminalPayload{
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

func logAgentStreamError(cfg *agentStreamConfig, phase string, err error, closeInfo *router.StreamCloseInfo) {
	if cfg.log == nil {
		return
	}
	cfg.log.Warn(
		"agent stream terminated with error",
		"exec_id", cfg.execID,
		"phase", phase,
		"error", err,
	)
	if closeInfo != nil && err != nil && closeInfo.Error == nil {
		closeInfo.Reason = fmt.Sprintf("%s_error", phase)
		closeInfo.Error = err
	}
}

func logAgentCompletion(cfg *agentStreamConfig, lastID int64, status core.StatusType) {
	if cfg.log == nil {
		return
	}
	cfg.log.Info(
		"Agent stream disconnected",
		"exec_id", cfg.execID,
		"last_event_id", lastID,
		"status", status,
	)
}

func logAgentCancellation(cfg *agentStreamConfig) {
	if cfg.log == nil {
		return
	}
	cfg.log.Info("Agent stream canceled", "exec_id", cfg.execID)
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

func handleAgentChunk(
	stream *router.SSEStream,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
	msg pubsub.Message,
	ok bool,
) (int64, bool) {
	if !ok {
		logAgentStreamError(cfg, "pubsub", errors.New("message channel closed"), closeInfo)
		return lastID, false
	}
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, llmChunkEvent, msg.Payload); err != nil {
		logAgentStreamError(cfg, "chunk", err, closeInfo)
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

func handleAgentPoll(
	ctx context.Context,
	stream *router.SSEStream,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
	lastStatus *core.StatusType,
	lastUpdated *time.Time,
) (int64, bool) {
	state, err := cfg.repo.GetState(ctx, cfg.execID)
	if err != nil {
		logAgentStreamError(cfg, "poll", err, closeInfo)
		return lastID, false
	}
	updatedID := lastID
	if state.Status != *lastStatus || state.UpdatedAt.After(*lastUpdated) {
		updatedID, err = emitAgentStatus(stream, telemetry, state, lastID)
		if err != nil {
			logAgentStreamError(cfg, "status", err, closeInfo)
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
		updatedID, err = emitAgentTerminal(stream, telemetry, state, updatedID)
		if err != nil {
			logAgentStreamError(cfg, "terminal", err, closeInfo)
			return lastID, false
		}
		if closeInfo != nil {
			closeInfo.LastEventID = updatedID
			closeInfo.Status = state.Status
			closeInfo.Reason = router.StreamReasonTerminal
		}
		logAgentCompletion(cfg, updatedID, state.Status)
		return updatedID, false
	}
	return updatedID, true
}

type agentStatusPayload struct {
	Status    core.StatusType `json:"status"`
	UpdatedAt time.Time       `json:"ts"`
}

type agentTerminalPayload struct {
	Status    core.StatusType      `json:"status"`
	Result    *core.Output         `json:"result,omitempty"`
	Error     *core.Error          `json:"error,omitempty"`
	Usage     *router.UsageSummary `json:"usage,omitempty"`
	UpdatedAt time.Time            `json:"ts"`
}
