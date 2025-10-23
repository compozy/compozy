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
	"github.com/compozy/compozy/engine/streaming"
	taskdomain "github.com/compozy/compozy/engine/task"
	taskuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/config"
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
	taskReplayLimit         = 500
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
	execID        core.ID
	repo          taskdomain.Repository
	pubsub        pubsub.Provider
	initial       *taskdomain.State
	pollInterval  time.Duration
	lastEventID   int64
	mode          taskStreamMode
	log           logger.Logger
	metrics       *monitoring.StreamingMetrics
	heartbeat     time.Duration
	events        map[string]struct{}
	channelPrefix string
	publisher     streaming.Publisher
}

type taskStreamTunables struct {
	defaultPoll   time.Duration
	minPoll       time.Duration
	maxPoll       time.Duration
	heartbeat     time.Duration
	channelPrefix string
}

type taskStreamDeps struct {
	state         *appstate.State
	repo          taskdomain.Repository
	resourceStore resources.ResourceStore
	execState     *taskdomain.State
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

func createTaskStreamFinalize(
	ctx context.Context,
	cfg *taskStreamConfig,
	closeInfo *router.StreamCloseInfo,
	started time.Time,
) func() {
	return func() {
		log := logger.FromContext(ctx)
		if log == nil {
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
			log.Warn("task stream terminated", append(fields, "error", closeInfo.Error)...)
			return
		}
		log.Info("Task stream disconnected", fields...)
	}
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
	telemetry := router.NewStreamTelemetry(ctx, monitoring.ExecutionKindTask, cfg.execID, cfg.metrics)
	if telemetry != nil {
		telemetry.Connected(cfg.lastEventID, "Task stream connected", "mode", cfg.mode.String())
		return telemetry.Context(), telemetry, closeInfo, func() {
			telemetry.Close(closeInfo)
		}
	}
	started := time.Now()
	log := logger.FromContext(ctx)
	if log != nil {
		log.Info(
			"Task stream connected",
			"exec_id", cfg.execID,
			"mode", cfg.mode.String(),
			"last_event_id", cfg.lastEventID,
		)
	}
	finalize := createTaskStreamFinalize(ctx, cfg, closeInfo, started)
	return ctx, nil, closeInfo, finalize
}

func resolveTaskStreamDependencies(ctx context.Context, c *gin.Context, execID core.ID) (*taskStreamDeps, bool) {
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
	return &taskStreamDeps{
		state:         state,
		repo:          repo,
		resourceStore: resourceStore,
		execState:     execState,
	}, true
}

func prepareTaskStreamConfig(ctx context.Context, c *gin.Context, execID core.ID) (*taskStreamConfig, bool) {
	deps, ok := resolveTaskStreamDependencies(ctx, c, execID)
	if !ok {
		return nil, false
	}
	tunables := resolveTaskStreamTunables(ctx)
	pollInterval, ok := parseTaskPollIntervalParam(c, tunables)
	if !ok {
		return nil, false
	}
	lastEventID, ok := parseTaskLastEventID(c)
	if !ok {
		return nil, false
	}
	events, ok := parseTaskEventsParam(c)
	if !ok {
		return nil, false
	}
	mode, err := determineTaskStreamMode(ctx, deps.state, deps.resourceStore, deps.execState)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusInternalServerError, "failed to resolve task stream mode", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, false
	}
	pubsubProvider, ok := resolveTaskPubSubDependency(c, deps.state, mode)
	if !ok {
		return nil, false
	}
	log := logger.FromContext(ctx)
	metrics := router.ResolveStreamingMetrics(c, deps.state)
	publisher, _ := deps.state.StreamPublisher()
	return &taskStreamConfig{
		execID:        execID,
		repo:          deps.repo,
		pubsub:        pubsubProvider,
		initial:       deps.execState,
		pollInterval:  pollInterval,
		lastEventID:   lastEventID,
		mode:          mode,
		log:           log,
		metrics:       metrics,
		heartbeat:     tunables.heartbeat,
		events:        events,
		channelPrefix: tunables.channelPrefix,
		publisher:     publisher,
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

func parseTaskPollIntervalParam(c *gin.Context, tunables taskStreamTunables) (time.Duration, bool) {
	interval, err := parseTaskPollInterval(c.Query("poll_ms"), tunables)
	if err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid poll interval", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return 0, false
	}
	return interval, true
}

func parseTaskEventsParam(c *gin.Context) (map[string]struct{}, bool) {
	raw := strings.TrimSpace(c.Query("events"))
	if raw == "" {
		return nil, true
	}
	allowed := map[string]struct{}{
		taskStatusEvent: {},
		llmChunkEvent:   {},
		completeEvent:   {},
		errorEvent:      {},
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

func parseTaskPollInterval(raw string, tunables taskStreamTunables) (time.Duration, error) {
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

func resolveTaskStreamTunables(ctx context.Context) taskStreamTunables {
	values := taskStreamTunables{
		defaultPoll:   taskStreamDefaultPoll,
		minPoll:       taskStreamMinPoll,
		maxPoll:       taskStreamMaxPoll,
		heartbeat:     taskStreamHeartbeatFreq,
		channelPrefix: taskdomain.DefaultStreamChannelPrefix,
	}
	cfg := config.FromContext(ctx)
	if cfg != nil {
		if poll := cfg.Stream.Task.DefaultPoll; poll > 0 {
			values.defaultPoll = poll
		}
		if minPoll := cfg.Stream.Task.MinPoll; minPoll > 0 {
			values.minPoll = minPoll
		}
		if maxPoll := cfg.Stream.Task.MaxPoll; maxPoll > 0 {
			values.maxPoll = maxPoll
		}
		if hb := cfg.Stream.Task.HeartbeatFrequency; hb > 0 {
			values.heartbeat = hb
		}
		if prefix := strings.TrimSpace(cfg.Stream.Task.RedisChannelPrefix); prefix != "" {
			values.channelPrefix = prefix
		}
	}
	if values.minPoll <= 0 {
		values.minPoll = taskStreamMinPoll
	}
	if values.maxPoll <= 0 {
		values.maxPoll = taskStreamMaxPoll
	}
	if values.defaultPoll <= 0 {
		values.defaultPoll = taskStreamDefaultPoll
	}
	if values.heartbeat <= 0 {
		values.heartbeat = taskStreamHeartbeatFreq
	}
	if strings.TrimSpace(values.channelPrefix) == "" {
		values.channelPrefix = taskdomain.DefaultStreamChannelPrefix
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

func shouldEmit(cfg *taskStreamConfig, event string) bool {
	if cfg == nil {
		return true
	}
	if len(cfg.events) == 0 {
		return true
	}
	_, ok := cfg.events[event]
	return ok
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
	if cfg.publisher == nil {
		switch cfg.mode {
		case streamModeStructured:
			runTaskStructuredStream(ctx, cfg, stream, telemetry, closeInfo)
		default:
			runTaskTextStream(ctx, cfg, stream, telemetry, closeInfo)
		}
	} else {
		runTaskEventStream(ctx, cfg, stream, telemetry, closeInfo)
	}
	if closeInfo.Reason == router.StreamReasonInitializing {
		closeInfo.Reason = router.StreamReasonCompleted
	}
}

func runTaskEventStream(
	ctx context.Context,
	cfg *taskStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
) {
	if cfg.pubsub == nil {
		runTaskStructuredStream(ctx, cfg, stream, telemetry, closeInfo)
		return
	}
	nextID, cont := emitTaskInitialEvents(stream, telemetry, cfg, cfg.initial, cfg.lastEventID, closeInfo)
	if !cont {
		if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
			closeInfo.Reason = router.StreamReasonTerminal
		}
		return
	}
	updatedID, ok := emitTaskReplayEvents(ctx, cfg, stream, telemetry, closeInfo, nextID)
	if !ok {
		return
	}
	subscription, err := cfg.pubsub.Subscribe(ctx, cfg.publisher.Channel(cfg.execID))
	if err != nil {
		logTaskStreamError(cfg, "subscribe", err, closeInfo)
		return
	}
	defer subscription.Close()
	taskEventLoop(ctx, cfg, telemetry, closeInfo, stream, subscription, updatedID)
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
	heartbeat := time.NewTicker(cfg.heartbeat)
	defer ticker.Stop()
	defer heartbeat.Stop()
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	for {
		select {
		case <-ctx.Done():
			if closeInfo != nil {
				closeInfo.Reason = router.StreamReasonContextCanceled
				closeInfo.Error = nil
			}
			logTaskCancellation(cfg)
			return
		case <-heartbeat.C:
			if !taskStreamHeartbeatTick(stream, telemetry, cfg, closeInfo) {
				return
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
	subscription, err := cfg.pubsub.Subscribe(ctx, redisTokenChannel(cfg.channelPrefix, cfg.execID))
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
	taskTextStreamLoop(ctx, cfg, telemetry, closeInfo, stream, subscription, nextID)
}

func taskTextStreamLoop(
	ctx context.Context,
	cfg *taskStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	stream *router.SSEStream,
	sub pubsub.Subscription,
	startID int64,
) {
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(cfg.heartbeat)
	defer ticker.Stop()
	defer heartbeat.Stop()
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	nextID := startID
	for {
		select {
		case <-ctx.Done():
			if closeInfo != nil {
				closeInfo.Reason = router.StreamReasonContextCanceled
				closeInfo.Error = nil
			}
			logTaskCancellation(cfg)
			return
		case <-heartbeat.C:
			if !taskStreamHeartbeatTick(stream, telemetry, cfg, closeInfo) {
				return
			}
		case <-sub.Done():
			taskStreamHandleSubscriptionDone(sub, cfg, closeInfo)
			return
		case msg, ok := <-sub.Messages():
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

func taskStreamHeartbeatTick(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	cfg *taskStreamConfig,
	closeInfo *router.StreamCloseInfo,
) bool {
	if err := stream.WriteHeartbeat(); err != nil {
		logTaskStreamError(cfg, "heartbeat", err, closeInfo)
		return false
	}
	if telemetry != nil {
		telemetry.RecordHeartbeat()
	}
	return true
}

func taskStreamHandleSubscriptionDone(
	sub pubsub.Subscription,
	cfg *taskStreamConfig,
	closeInfo *router.StreamCloseInfo,
) {
	if err := sub.Err(); err != nil {
		logTaskStreamError(cfg, "pubsub", err, closeInfo)
		return
	}
	if closeInfo != nil && closeInfo.Reason == router.StreamReasonInitializing {
		closeInfo.Reason = router.StreamReasonTerminal
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
	nextID, err := emitTaskStatus(stream, telemetry, cfg, state, lastID)
	if err != nil {
		logTaskStreamError(cfg, "status", err, closeInfo)
		return lastID, false
	}
	if !isTerminalStatus(state.Status) {
		if closeInfo != nil {
			closeInfo.LastEventID = nextID
			closeInfo.Status = state.Status
		}
		return nextID, true
	}
	updatedID, err := emitTaskTerminal(stream, telemetry, cfg, state, nextID)
	if err != nil {
		logTaskStreamError(cfg, "terminal", err, closeInfo)
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

func emitTaskReplayEvents(
	ctx context.Context,
	cfg *taskStreamConfig,
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	lastID int64,
) (int64, bool) {
	if cfg.publisher == nil {
		return lastID, true
	}
	envelopes, err := cfg.publisher.Replay(ctx, cfg.execID, lastID, taskReplayLimit)
	if err != nil {
		logTaskStreamError(cfg, "replay", err, closeInfo)
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
			updateTaskStreamCloseInfo(closeInfo, &env)
			continue
		}
		if err := stream.WriteEvent(env.ID, eventType, env.Data); err != nil {
			logTaskStreamError(cfg, "replay", err, closeInfo)
			return nextID, false
		}
		if telemetry != nil {
			telemetry.RecordEvent(eventType, true)
		}
		nextID = env.ID
		updateTaskStreamCloseInfo(closeInfo, &env)
	}
	return nextID, true
}

func taskEventLoop(
	ctx context.Context,
	cfg *taskStreamConfig,
	telemetry router.StreamTelemetry,
	closeInfo *router.StreamCloseInfo,
	stream *router.SSEStream,
	subscription pubsub.Subscription,
	startID int64,
) {
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
			if closeInfo != nil {
				closeInfo.Reason = router.StreamReasonContextCanceled
				closeInfo.Error = nil
			}
			logTaskCancellation(cfg)
			return
		case <-heartbeat.C:
			if !taskStreamHeartbeatTick(stream, telemetry, cfg, closeInfo) {
				return
			}
		case msg, ok := <-messages:
			updatedID, ok := handleTaskEvent(stream, cfg, telemetry, closeInfo, nextID, msg, ok)
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
		case <-subscription.Done():
			if err := subscription.Err(); err != nil {
				logTaskStreamError(cfg, "pubsub", err, closeInfo)
			}
			return
		}
	}
}

func emitTaskStatus(
	stream *router.SSEStream,
	telemetry router.StreamTelemetry,
	cfg *taskStreamConfig,
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
	if !shouldEmit(cfg, taskStatusEvent) {
		return lastID, nil
	}
	nextID := lastID
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
	cfg *taskStreamConfig,
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
	if !shouldEmit(cfg, eventType) {
		return lastID, nil
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
		"error", core.RedactError(err),
	)
	if closeInfo != nil && err != nil && closeInfo.Error == nil {
		closeInfo.Reason = fmt.Sprintf("%s_error", phase)
		closeInfo.Error = fmt.Errorf("%s", core.RedactError(err))
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

func redisTokenChannel(prefix string, execID core.ID) string {
	return fmt.Sprintf("%s%s", prefix, execID.String())
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
	if !shouldEmit(cfg, llmChunkEvent) {
		if closeInfo != nil {
			closeInfo.LastEventID = lastID
		}
		return lastID, true
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

func decodeEnvelope(payload []byte) (streaming.Envelope, error) {
	var envelope streaming.Envelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return streaming.Envelope{}, err
	}
	return envelope, nil
}

func handleTaskEvent(
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
	envelope, err := decodeEnvelope(msg.Payload)
	if err != nil {
		logTaskStreamError(cfg, "decode", err, closeInfo)
		return lastID, true
	}
	if envelope.ID <= lastID {
		updateTaskStreamCloseInfo(closeInfo, &envelope)
		return lastID, true
	}
	eventType := string(envelope.Type)
	if !shouldEmit(cfg, eventType) {
		updateTaskStreamCloseInfo(closeInfo, &envelope)
		return envelope.ID, true
	}
	if err := stream.WriteEvent(envelope.ID, eventType, envelope.Data); err != nil {
		logTaskStreamError(cfg, "event", err, closeInfo)
		return lastID, false
	}
	if telemetry != nil {
		telemetry.RecordEvent(eventType, true)
	}
	updateTaskStreamCloseInfo(closeInfo, &envelope)
	return envelope.ID, true
}

func updateTaskStreamCloseInfo(info *router.StreamCloseInfo, env *streaming.Envelope) {
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
		updatedID, err = emitTaskStatus(stream, telemetry, cfg, state, lastID)
		if err != nil {
			logTaskStreamError(cfg, "status", err, closeInfo)
			return lastID, false
		}
		*lastStatus = state.Status
		*lastUpdated = state.UpdatedAt
		if closeInfo != nil {
			if updatedID != lastID {
				closeInfo.LastEventID = updatedID
			}
			closeInfo.Status = state.Status
		}
	}
	if isTerminalStatus(state.Status) {
		updatedID, err = emitTaskTerminal(stream, telemetry, cfg, state, updatedID)
		if err != nil {
			logTaskStreamError(cfg, "terminal", err, closeInfo)
			return lastID, false
		}
		if closeInfo != nil {
			if updatedID != lastID {
				closeInfo.LastEventID = updatedID
			}
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
