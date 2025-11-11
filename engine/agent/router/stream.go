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
	"github.com/compozy/compozy/engine/streaming"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	defaultAgentStreamPoll      = 500 * time.Millisecond
	defaultAgentStreamMinPoll   = 250 * time.Millisecond
	defaultAgentStreamMaxPoll   = 2000 * time.Millisecond
	defaultAgentStreamHeartbeat = 15 * time.Second
	agentStatusEvent            = "agent_status"
	completeEvent               = "complete"
	errorEvent                  = "error"
	llmChunkEvent               = "llm_chunk"
	promptActionID              = "__prompt__"
	defaultAgentReplayLimit     = 500
)

type agentStreamTunables struct {
	defaultPoll time.Duration
	minPoll     time.Duration
	maxPoll     time.Duration
	heartbeat   time.Duration
	replayLimit int
}

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
	heartbeat    time.Duration
	lastEventID  int64
	mode         streamMode
	metrics      *monitoring.StreamingMetrics
	events       map[string]struct{}
	publisher    streaming.Publisher
	replayLimit  int
}

type agentStreamDeps struct {
	state         *appstate.State
	repo          task.Repository
	resourceStore resources.ResourceStore
	tunables      agentStreamTunables
}

type agentRequestParams struct {
	pollInterval time.Duration
	lastEventID  int64
	events       map[string]struct{}
}

type agentLoop struct {
	ctx       context.Context
	cfg       *agentStreamConfig
	stream    *router.SSEStream
	telemetry router.StreamTelemetry
	closeInfo *router.StreamCloseInfo
}

// streamAgentExecution streams Server-Sent Events for agent executions.
//
//	@Summary		Stream agent execution events
//	@Description	Streams agent execution updates over Server-Sent Events, emitting structured JSON or llm_chunk text depending on the output schema. Served under routes.Base() (e.g., /api/v0/executions/agents/{exec_id}/stream).
//	@Tags			executions
//	@Accept			*/*
//	@Produce		text/event-stream
//	@Param			exec_id	path		string											true	"Agent execution ID"	example("2Z4PVTL6K27XVT4A3NPKMDD5BG")
//	@Param			Last-Event-ID	header		string											false	"Resume the stream from the provided event id"	example("42")
//	@Param			poll_ms			query		int												false	"Polling interval (milliseconds). Default 500, min 250, max 2000."	example(500)
//	@Param			events			query		string											false	"Comma-separated list of event types to emit (default: all events)."	example("agent_status,llm_chunk,complete")
//	@Success		200				{string}	string											"SSE stream"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}			"Invalid request"
//	@Failure		404				{object}	router.Response{error=router.ErrorInfo}			"Execution not found"
//	@Failure		503				{object}	router.Response{error=router.ErrorInfo}			"Pub/Sub provider unavailable"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}			"Internal server error"
//	@Router			/executions/agents/{exec_id}/stream [get]
func streamAgentExecution(c *gin.Context) {
	execID, ok := parseAgentExecID(c)
	if !ok {
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
	telemetry := router.NewStreamTelemetry(ctx, monitoring.ExecutionKindAgent, cfg.execID, cfg.metrics)
	if telemetry != nil {
		telemetry.Connected(cfg.lastEventID, "Agent stream connected", "mode", cfg.mode.String())
		return telemetry.Context(), telemetry, closeInfo, func() {
			telemetry.Close(closeInfo)
		}
	}
	started := time.Now()
	log := logger.FromContext(ctx)
	if log != nil {
		log.Info(
			"Agent stream connected",
			"exec_id", cfg.execID,
			"mode", cfg.mode.String(),
			"last_event_id", cfg.lastEventID,
		)
	}
	return ctx, nil, closeInfo, buildAgentFinalize(ctx, started, cfg.execID, closeInfo)
}

func prepareAgentStreamConfig(ctx context.Context, c *gin.Context, execID core.ID) (*agentStreamConfig, bool) {
	deps, ok := resolveAgentStreamDependencies(ctx, c)
	if !ok {
		return nil, false
	}
	taskState, ok := fetchAgentTaskState(ctx, c, deps.repo, execID)
	if !ok {
		return nil, false
	}
	params, ok := parseAgentRequestParams(c, deps.tunables)
	if !ok {
		return nil, false
	}
	mode, err := determineAgentStreamMode(ctx, deps.state, deps.resourceStore, taskState)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to resolve agent stream mode", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	provider, ok := resolveAgentPubSubDependency(c, deps.state, mode)
	if !ok {
		return nil, false
	}
	metrics := router.ResolveStreamingMetrics(c, deps.state)
	publisher, _ := deps.state.StreamPublisher()
	return &agentStreamConfig{
		execID:       execID,
		repo:         deps.repo,
		pubsub:       provider,
		initial:      taskState,
		pollInterval: params.pollInterval,
		heartbeat:    deps.tunables.heartbeat,
		lastEventID:  params.lastEventID,
		mode:         mode,
		metrics:      metrics,
		events:       params.events,
		publisher:    publisher,
		replayLimit:  deps.tunables.replayLimit,
	}, true
}

func resolveAgentStreamDependencies(ctx context.Context, c *gin.Context) (*agentStreamDeps, bool) {
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
	store, ok := router.GetResourceStore(c)
	if !ok {
		return nil, false
	}
	return &agentStreamDeps{
		state:         state,
		repo:          repo,
		resourceStore: store,
		tunables:      resolveAgentStreamTunables(ctx),
	}, true
}

func parseAgentRequestParams(c *gin.Context, tunables agentStreamTunables) (*agentRequestParams, bool) {
	poll, ok := parseAgentPollIntervalParam(c, tunables)
	if !ok {
		return nil, false
	}
	lastEventID, ok := parseAgentLastEventID(c)
	if !ok {
		return nil, false
	}
	events, ok := parseAgentEventsParam(c)
	if !ok {
		return nil, false
	}
	return &agentRequestParams{pollInterval: poll, lastEventID: lastEventID, events: events}, true
}

func parseAgentExecID(c *gin.Context) (core.ID, bool) {
	execID := router.GetAgentExecID(c)
	if execID == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "missing execution id", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return "", false
	}
	if _, err := core.ParseID(execID.String()); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid execution id", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return "", false
	}
	return execID, true
}

func (l *agentLoop) cancel() {
	if l.closeInfo != nil {
		l.closeInfo.Reason = router.StreamReasonContextCanceled
		l.closeInfo.Error = nil
	}
	logAgentCancellation(l.ctx, l.cfg)
}

func (l *agentLoop) handleHeartbeat() bool {
	if err := l.stream.WriteHeartbeat(); err != nil {
		logAgentStreamError(l.ctx, l.cfg, "heartbeat", err, l.closeInfo)
		return false
	}
	if l.telemetry != nil {
		l.telemetry.RecordHeartbeat()
	}
	return true
}

func (l *agentLoop) handlePoll(nextID int64, lastStatus *core.StatusType, lastUpdated *time.Time) (int64, bool) {
	return handleAgentPoll(l.ctx, l.stream, l.cfg, l.telemetry, l.closeInfo, nextID, lastStatus, lastUpdated)
}

func (l *agentLoop) handleEvent(lastID int64, msg pubsub.Message, ok bool) (int64, bool) {
	return handleAgentEvent(l.ctx, l.stream, l.cfg, l.telemetry, l.closeInfo, lastID, msg, ok)
}

func (l *agentLoop) handleChunk(lastID int64, msg pubsub.Message, ok bool) (int64, bool) {
	return handleAgentChunk(l.ctx, l.stream, l.cfg, l.telemetry, l.closeInfo, lastID, msg, ok)
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

func parseAgentPollIntervalParam(c *gin.Context, tunables agentStreamTunables) (time.Duration, bool) {
	interval, err := parseAgentPollInterval(c.Query("poll_ms"), tunables)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid poll interval", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return interval, true
}

func parseAgentPollInterval(raw string, tunables agentStreamTunables) (time.Duration, error) {
	if raw == "" {
		return tunables.defaultPoll, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid poll_ms: %w", err)
	}
	if ms < int(tunables.minPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be >= %d", tunables.minPoll/time.Millisecond)
	}
	if ms > int(tunables.maxPoll/time.Millisecond) {
		return 0, fmt.Errorf("poll_ms must be <= %d", tunables.maxPoll/time.Millisecond)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func resolveAgentStreamTunables(ctx context.Context) agentStreamTunables {
	values := agentStreamTunables{
		defaultPoll: defaultAgentStreamPoll,
		minPoll:     defaultAgentStreamMinPoll,
		maxPoll:     defaultAgentStreamMaxPoll,
		heartbeat:   defaultAgentStreamHeartbeat,
		replayLimit: defaultAgentReplayLimit,
	}
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return normalizeAgentStreamTunables(values)
	}
	if poll := cfg.Stream.Agent.DefaultPoll; poll > 0 {
		values.defaultPoll = poll
	}
	if minPoll := cfg.Stream.Agent.MinPoll; minPoll > 0 {
		values.minPoll = minPoll
	}
	if maxPoll := cfg.Stream.Agent.MaxPoll; maxPoll > 0 {
		values.maxPoll = maxPoll
	}
	if hb := cfg.Stream.Agent.HeartbeatFrequency; hb > 0 {
		values.heartbeat = hb
	}
	if limit := cfg.Stream.Agent.ReplayLimit; limit > 0 {
		values.replayLimit = limit
	}
	return normalizeAgentStreamTunables(values)
}

func normalizeAgentStreamTunables(values agentStreamTunables) agentStreamTunables {
	if values.minPoll <= 0 {
		values.minPoll = defaultAgentStreamMinPoll
	}
	if values.maxPoll <= 0 {
		values.maxPoll = defaultAgentStreamMaxPoll
	}
	if values.defaultPoll <= 0 {
		values.defaultPoll = defaultAgentStreamPoll
	}
	if values.heartbeat <= 0 {
		values.heartbeat = defaultAgentStreamHeartbeat
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
	if values.replayLimit <= 0 {
		values.replayLimit = defaultAgentReplayLimit
	}
	return values
}

func parseAgentEventsParam(c *gin.Context) (map[string]struct{}, bool) {
	raw := strings.TrimSpace(c.Query("events"))
	if raw == "" {
		return nil, true
	}
	allowed := map[string]struct{}{
		agentStatusEvent: {},
		llmChunkEvent:    {},
		completeEvent:    {},
		errorEvent:       {},
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
		reqErr := router.NewRequestError(http.StatusServiceUnavailable, "pub/sub provider unavailable", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	return provider, true
}

func runAgentStream(ctx context.Context, cfg *agentStreamConfig, stream *router.SSEStream) {
	ctx, telemetry, closeInfo, finalize := initAgentTelemetry(ctx, cfg)
	defer finalize()
	if cfg.publisher == nil {
		switch cfg.mode {
		case streamModeStructured:
			runAgentStructuredStream(ctx, cfg, stream, telemetry, closeInfo)
		default:
			runAgentTextStream(ctx, cfg, stream, telemetry, closeInfo)
		}
	} else {
		runAgentEventStream(ctx, cfg, stream, telemetry, closeInfo)
	}
	if closeInfo.Reason == router.StreamReasonInitializing {
		closeInfo.Reason = router.StreamReasonCompleted
	}
}

func runAgentEventStream(
	ctx context.Context,
	cfg *agentStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
) {
	if cfg.pubsub == nil {
		runAgentStructuredStream(ctx, cfg, stream, telemetry, closeInfo)
		return
	}
	nextID, cont := emitAgentInitialEvents(ctx, stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	updatedID, ok := emitAgentReplayEvents(ctx, cfg, stream, telemetry, closeInfo, nextID)
	if !ok {
		return
	}
	subscription, err := cfg.pubsub.Subscribe(ctx, cfg.publisher.Channel(cfg.execID))
	if err != nil {
		logAgentStreamError(ctx, cfg, "subscribe", err, closeInfo)
		return
	}
	defer subscription.Close()
	agentEventLoop(ctx, cfg, telemetry, closeInfo, stream, subscription, updatedID)
}

func runAgentStructuredStream(
	ctx context.Context,
	cfg *agentStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
) {
	nextID, cont := emitAgentInitialEvents(ctx, stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	loop := agentLoop{ctx: ctx, cfg: cfg, stream: stream, telemetry: telemetry, closeInfo: closeInfo}
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(cfg.heartbeat)
	defer ticker.Stop()
	defer heartbeat.Stop()
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	for {
		select {
		case <-ctx.Done():
			loop.cancel()
			return
		case <-heartbeat.C:
			if !loop.handleHeartbeat() {
				return
			}
		case <-ticker.C:
			updatedID, ok := loop.handlePoll(nextID, &lastStatus, &lastUpdated)
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
		logAgentStreamError(ctx, cfg, "subscribe", err, closeInfo)
		return
	}
	defer subscription.Close()
	nextID, cont := emitAgentInitialEvents(ctx, stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	agentTextStreamLoop(ctx, cfg, telemetry, closeInfo, stream, subscription, nextID)
}

func agentTextStreamLoop(
	ctx context.Context,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	stream *router.SSEStream,
	subscription pubsub.Subscription,
	startID int64,
) {
	loop := agentLoop{ctx: ctx, cfg: cfg, stream: stream, telemetry: telemetry, closeInfo: closeInfo}
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(cfg.heartbeat)
	defer ticker.Stop()
	defer heartbeat.Stop()
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	nextID := startID
	messages := subscription.Messages()
	for {
		select {
		case <-ctx.Done():
			loop.cancel()
			return
		case <-heartbeat.C:
			if !loop.handleHeartbeat() {
				return
			}
		case msg, ok := <-messages:
			updatedID, ok := loop.handleChunk(nextID, msg, ok)
			if !ok {
				return
			}
			nextID = updatedID
		case <-ticker.C:
			updatedID, ok := loop.handlePoll(nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		case <-subscription.Done():
			if err := subscription.Err(); err != nil {
				logAgentStreamError(ctx, cfg, "pubsub", err, closeInfo)
			}
			return
		}
	}
}

func emitAgentInitialEvents(
	ctx context.Context,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	cfg *agentStreamConfig,
	state *task.State,
	lastID int64,
	closeInfo *router.StreamCloseInfo,
) (int64, bool) {
	nextID := lastID
	if shouldEmit(cfg, agentStatusEvent) {
		statusID, err := emitAgentStatus(stream, telemetry, state, lastID)
		if err != nil {
			logAgentStreamError(ctx, cfg, "status", err, closeInfo)
			return lastID, false
		}
		nextID = statusID
		if closeInfo != nil {
			closeInfo.LastEventID = statusID
			closeInfo.Status = state.Status
		}
	} else if closeInfo != nil {
		closeInfo.Status = state.Status
	}
	if !isTerminalStatus(state.Status) {
		return nextID, true
	}
	eventType := terminalEventType(state.Status)
	if shouldEmit(cfg, eventType) {
		updatedID, termErr := emitAgentTerminal(stream, telemetry, state, nextID)
		if termErr != nil {
			logAgentStreamError(ctx, cfg, "terminal", termErr, closeInfo)
			return nextID, false
		}
		nextID = updatedID
	}
	if closeInfo != nil {
		closeInfo.LastEventID = nextID
		closeInfo.Status = state.Status
		closeInfo.Reason = router.StreamReasonTerminal
	}
	logAgentCompletion(ctx, cfg, nextID, state.Status)
	return nextID, false
}

func emitAgentReplayEvents(
	ctx context.Context,
	cfg *agentStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
) (int64, bool) {
	if cfg.publisher == nil {
		return lastID, true
	}
	limit := cfg.replayLimit
	if limit <= 0 {
		limit = defaultAgentReplayLimit
	}
	envelopes, err := cfg.publisher.Replay(ctx, cfg.execID, lastID, limit)
	if err != nil {
		logAgentStreamError(ctx, cfg, "replay", err, closeInfo)
		return lastID, false
	}
	nextID := lastID
	for _, env := range envelopes {
		if env.ID <= nextID {
			continue
		}
		eventType := string(env.Type)
		if !shouldEmit(cfg, eventType) {
			nextID = env.ID
			continue
		}
		if err := stream.WriteEvent(env.ID, eventType, env.Data); err != nil {
			logAgentStreamError(ctx, cfg, "replay", err, closeInfo)
			return nextID, false
		}
		if telemetry != nil {
			telemetry.RecordEvent(eventType, true)
		}
		nextID = env.ID
		updateAgentStreamCloseInfo(closeInfo, &env)
	}
	return nextID, true
}

func agentEventLoop(
	ctx context.Context,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	stream *router.SSEStream,
	subscription pubsub.Subscription,
	startID int64,
) {
	loop := agentLoop{ctx: ctx, cfg: cfg, stream: stream, telemetry: telemetry, closeInfo: closeInfo}
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(cfg.heartbeat)
	defer ticker.Stop()
	defer heartbeat.Stop()
	nextID := startID
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	messages := subscription.Messages()
	for {
		select {
		case <-ctx.Done():
			loop.cancel()
			return
		case <-heartbeat.C:
			if !loop.handleHeartbeat() {
				return
			}
		case msg, ok := <-messages:
			updatedID, ok := loop.handleEvent(nextID, msg, ok)
			if !ok {
				return
			}
			nextID = updatedID
		case <-ticker.C:
			updatedID, ok := loop.handlePoll(nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		case <-subscription.Done():
			if err := subscription.Err(); err != nil {
				logAgentStreamError(ctx, cfg, "pubsub", err, closeInfo)
			}
			return
		}
	}
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
	eventType := terminalEventType(state.Status)
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, eventType, data); err != nil {
		return lastID, err
	}
	if telemetry != nil {
		telemetry.RecordEvent(eventType, true)
	}
	return nextID, nil
}

func logAgentStreamError(
	ctx context.Context,
	cfg *agentStreamConfig,
	phase string,
	err error,
	closeInfo *router.StreamCloseInfo,
) {
	log := logger.FromContext(ctx)
	if log != nil {
		log.Warn(
			"agent stream terminated with error",
			"exec_id", cfg.execID,
			"phase", phase,
			"error", core.RedactError(err),
		)
	}
	if closeInfo != nil && err != nil && closeInfo.Error == nil {
		closeInfo.Reason = fmt.Sprintf("%s_error", phase)
		closeInfo.Error = fmt.Errorf("%s", core.RedactError(err))
	}
}

func logAgentCompletion(ctx context.Context, cfg *agentStreamConfig, lastID int64, status core.StatusType) {
	log := logger.FromContext(ctx)
	if log == nil {
		return
	}
	log.Info(
		"Agent stream disconnected",
		"exec_id", cfg.execID,
		"last_event_id", lastID,
		"status", status,
	)
}

func logAgentCancellation(ctx context.Context, cfg *agentStreamConfig) {
	log := logger.FromContext(ctx)
	if log != nil {
		log.Info("Agent stream canceled", "exec_id", cfg.execID)
	}
}

func buildAgentFinalize(ctx context.Context, started time.Time, execID core.ID, info *router.StreamCloseInfo) func() {
	return func() {
		log := logger.FromContext(ctx)
		if log == nil {
			return
		}
		fields := []any{"exec_id", execID, "duration", time.Since(started)}
		if info == nil {
			log.Info("Agent stream disconnected", fields...)
			return
		}
		fields = append(fields, "last_event_id", info.LastEventID, "reason", info.Reason)
		if info.Status != nil {
			fields = append(fields, "status", info.Status)
		}
		if len(info.ExtraFields) > 0 {
			fields = append(fields, info.ExtraFields...)
		}
		if info.Error != nil {
			log.Warn("agent stream terminated", append(fields, "error", core.RedactError(info.Error))...)
			return
		}
		log.Info("Agent stream disconnected", fields...)
	}
}

func isTerminalStatus(status core.StatusType) bool {
	switch status {
	case core.StatusSuccess, core.StatusFailed, core.StatusTimedOut, core.StatusCanceled:
		return true
	default:
		return false
	}
}

func terminalEventType(status core.StatusType) string {
	if status == core.StatusSuccess {
		return completeEvent
	}
	return errorEvent
}

func shouldEmit(cfg *agentStreamConfig, event string) bool {
	if cfg == nil {
		return true
	}
	if len(cfg.events) == 0 {
		return true
	}
	_, ok := cfg.events[event]
	return ok
}

func redisTokenChannel(execID core.ID) string {
	return fmt.Sprintf("%s%s", task.DefaultStreamChannelPrefix, execID.String())
}

func handleAgentChunk(
	ctx context.Context,
	stream *router.SSEStream,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
	msg pubsub.Message,
	ok bool,
) (int64, bool) {
	if !ok {
		logAgentStreamError(ctx, cfg, "pubsub", errors.New("message channel closed"), closeInfo)
		return lastID, false
	}
	if !shouldEmit(cfg, llmChunkEvent) {
		if closeInfo != nil {
			closeInfo.LastEventID = lastID
		}
		return lastID, true
	}
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, llmChunkEvent, msg.Payload); err != nil {
		logAgentStreamError(ctx, cfg, "chunk", err, closeInfo)
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

func handleAgentEvent(
	ctx context.Context,
	stream *router.SSEStream,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
	msg pubsub.Message,
	ok bool,
) (int64, bool) {
	if !ok {
		logAgentStreamError(ctx, cfg, "pubsub", errors.New("message channel closed"), closeInfo)
		return lastID, false
	}
	envelope, err := decodeEnvelope(msg.Payload)
	if err != nil {
		logAgentStreamError(ctx, cfg, "decode", err, closeInfo)
		return lastID, true
	}
	if envelope.ID <= lastID {
		updateAgentStreamCloseInfo(closeInfo, &envelope)
		return lastID, true
	}
	eventType := string(envelope.Type)
	if !shouldEmit(cfg, eventType) {
		return envelope.ID, true
	}
	if err := stream.WriteEvent(envelope.ID, eventType, envelope.Data); err != nil {
		logAgentStreamError(ctx, cfg, "event", err, closeInfo)
		return lastID, false
	}
	if telemetry != nil {
		telemetry.RecordEvent(eventType, true)
	}
	updateAgentStreamCloseInfo(closeInfo, &envelope)
	return envelope.ID, true
}

func decodeEnvelope(payload []byte) (streaming.Envelope, error) {
	var envelope streaming.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return streaming.Envelope{}, err
	}
	return envelope, nil
}

func updateAgentStreamCloseInfo(info *router.StreamCloseInfo, env *streaming.Envelope) {
	if info == nil || env == nil {
		return
	}
	info.LastEventID = env.ID
	if status, ok := decodeStreamStatus(env.Data); ok {
		info.Status = status
		if isTerminalStatus(status) && info.Reason == router.StreamReasonInitializing {
			info.Reason = router.StreamReasonTerminal
		}
	}
	switch env.Type {
	case streaming.EventTypeComplete:
		if info.Reason == router.StreamReasonInitializing {
			info.Reason = router.StreamReasonTerminal
		}
	case streaming.EventTypeError:
		info.Reason = router.StreamReasonStreamError
	}
}

func decodeStreamStatus(data json.RawMessage) (core.StatusType, bool) {
	if len(data) == 0 {
		return "", false
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", false
	}
	if value, ok := payload["status"].(string); ok {
		return core.StatusType(value), true
	}
	return "", false
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
		logAgentStreamError(ctx, cfg, "poll", err, closeInfo)
		return lastID, false
	}
	updatedID := lastID
	if state.Status != *lastStatus || state.UpdatedAt.After(*lastUpdated) {
		if shouldEmit(cfg, agentStatusEvent) {
			updatedID, err = emitAgentStatus(stream, telemetry, state, lastID)
			if err != nil {
				logAgentStreamError(ctx, cfg, "status", err, closeInfo)
				return lastID, false
			}
		} else {
			updatedID = lastID
		}
		*lastStatus = state.Status
		*lastUpdated = state.UpdatedAt
		if closeInfo != nil {
			closeInfo.LastEventID = updatedID
			closeInfo.Status = state.Status
		}
	}
	if isTerminalStatus(state.Status) {
		return handleAgentTerminalOnPoll(ctx, stream, cfg, telemetry, closeInfo, state, updatedID)
	}
	return updatedID, true
}

func handleAgentTerminalOnPoll(
	ctx context.Context,
	stream *router.SSEStream,
	cfg *agentStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	state *task.State,
	currentID int64,
) (int64, bool) {
	updatedID := currentID
	if shouldEmit(cfg, terminalEventType(state.Status)) {
		var err error
		updatedID, err = emitAgentTerminal(stream, telemetry, state, updatedID)
		if err != nil {
			logAgentStreamError(ctx, cfg, "terminal", err, closeInfo)
			return currentID, false
		}
	}
	if closeInfo != nil {
		closeInfo.LastEventID = updatedID
		closeInfo.Status = state.Status
		closeInfo.Reason = router.StreamReasonTerminal
	}
	logAgentCompletion(ctx, cfg, updatedID, state.Status)
	return updatedID, false
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
