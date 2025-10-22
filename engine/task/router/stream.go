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
}

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
	return &taskStreamConfig{
		execID:       execID,
		repo:         repo,
		pubsub:       pubsubProvider,
		initial:      execState,
		pollInterval: pollInterval,
		lastEventID:  lastEventID,
		mode:         mode,
		log:          log,
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
	if cfg.log != nil {
		cfg.log.Info(
			"Task stream connected",
			"exec_id", cfg.execID,
			"mode", cfg.mode.String(),
			"last_event_id", cfg.lastEventID,
		)
	}
	switch cfg.mode {
	case streamModeStructured:
		runTaskStructuredStream(ctx, cfg, stream)
	default:
		runTaskTextStream(ctx, cfg, stream)
	}
}

func runTaskStructuredStream(ctx context.Context, cfg *taskStreamConfig, stream *router.SSEStream) {
	nextID, cont := emitTaskInitialEvents(stream, cfg, cfg.initial, cfg.lastEventID)
	if !cont {
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
			logTaskCancellation(cfg)
			return
		case <-heartbeat.C:
			if err := stream.WriteHeartbeat(); err != nil {
				logTaskStreamError(cfg, "heartbeat", err)
				return
			}
		case <-ticker.C:
			updatedID, ok := handleTaskPoll(ctx, stream, cfg, nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		}
	}
}

func runTaskTextStream(ctx context.Context, cfg *taskStreamConfig, stream *router.SSEStream) {
	subscription, err := cfg.pubsub.Subscribe(ctx, redisTokenChannel(cfg.execID))
	if err != nil {
		logTaskStreamError(cfg, "subscribe", err)
		return
	}
	defer subscription.Close()
	nextID, cont := emitTaskInitialEvents(stream, cfg, cfg.initial, cfg.lastEventID)
	if !cont {
		return
	}
	messages := subscription.Messages()
	ticker := time.NewTicker(cfg.pollInterval)
	heartbeat := time.NewTicker(taskStreamHeartbeatFreq)
	defer ticker.Stop()
	defer heartbeat.Stop()
	lastStatus := cfg.initial.Status
	lastUpdated := cfg.initial.UpdatedAt
	for {
		select {
		case <-ctx.Done():
			logTaskCancellation(cfg)
			return
		case <-heartbeat.C:
			if err := stream.WriteHeartbeat(); err != nil {
				logTaskStreamError(cfg, "heartbeat", err)
				return
			}
		case msg, ok := <-messages:
			updatedID, ok := handleTaskChunk(stream, cfg, nextID, msg, ok)
			if !ok {
				return
			}
			nextID = updatedID
		case <-ticker.C:
			updatedID, ok := handleTaskPoll(ctx, stream, cfg, nextID, &lastStatus, &lastUpdated)
			if !ok {
				return
			}
			nextID = updatedID
		}
	}
}

func emitTaskInitialEvents(
	stream *router.SSEStream,
	cfg *taskStreamConfig,
	state *taskdomain.State,
	lastID int64,
) (int64, bool) {
	nextID, err := emitTaskStatus(stream, state, lastID)
	if err != nil {
		logTaskStreamError(cfg, "status", err)
		return lastID, false
	}
	if isTerminalStatus(state.Status) {
		updatedID, termErr := emitTaskTerminal(stream, state, nextID)
		if termErr != nil {
			logTaskStreamError(cfg, "terminal", termErr)
			return nextID, false
		}
		logTaskCompletion(cfg, updatedID, state.Status)
		return updatedID, false
	}
	return nextID, true
}

func emitTaskStatus(stream *router.SSEStream, state *taskdomain.State, lastID int64) (int64, error) {
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
	return nextID, nil
}

func emitTaskTerminal(stream *router.SSEStream, state *taskdomain.State, lastID int64) (int64, error) {
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
	return nextID, nil
}

func logTaskStreamError(cfg *taskStreamConfig, phase string, err error) {
	if cfg.log == nil {
		return
	}
	cfg.log.Warn(
		"task stream terminated with error",
		"exec_id", cfg.execID,
		"phase", phase,
		"error", err,
	)
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
	lastID int64,
	msg pubsub.Message,
	ok bool,
) (int64, bool) {
	if !ok {
		logTaskStreamError(cfg, "pubsub", errors.New("message channel closed"))
		return lastID, false
	}
	nextID := lastID + 1
	if err := stream.WriteEvent(nextID, llmChunkEvent, msg.Payload); err != nil {
		logTaskStreamError(cfg, "chunk", err)
		return lastID, false
	}
	return nextID, true
}

func handleTaskPoll(
	ctx context.Context,
	stream *router.SSEStream,
	cfg *taskStreamConfig,
	lastID int64,
	lastStatus *core.StatusType,
	lastUpdated *time.Time,
) (int64, bool) {
	state, err := cfg.repo.GetState(ctx, cfg.execID)
	if err != nil {
		logTaskStreamError(cfg, "poll", err)
		return lastID, false
	}
	updatedID := lastID
	if state.Status != *lastStatus || state.UpdatedAt.After(*lastUpdated) {
		updatedID, err = emitTaskStatus(stream, state, lastID)
		if err != nil {
			logTaskStreamError(cfg, "status", err)
			return lastID, false
		}
		*lastStatus = state.Status
		*lastUpdated = state.UpdatedAt
	}
	if isTerminalStatus(state.Status) {
		updatedID, err = emitTaskTerminal(stream, state, updatedID)
		if err != nil {
			logTaskStreamError(cfg, "terminal", err)
			return lastID, false
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
