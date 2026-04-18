package acpshared

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var marshalPublicSessionUpdateJSON = json.Marshal

type SessionUpdateHandler struct {
	ctx            context.Context
	index          int
	agentID        string
	jobID          string
	sessionID      string
	logger         *slog.Logger
	runID          string
	runManager     model.RuntimeManager
	startedAt      time.Time
	outWriter      io.Writer
	errWriter      io.Writer
	journal        runtimeEventSubmitter
	jobUsage       *model.Usage
	aggregateUsage *model.Usage
	aggregateMu    *sync.Mutex
	activity       *activityMonitor
	reusableAgent  *reusableAgentExecution

	mu                   sync.Mutex
	err                  error
	blockCounts          map[model.ContentBlockType]int
	nestedToolCalls      map[string]nestedReusableAgentCall
	pendingNestedResults map[string]runAgentToolResult
	sessionView          *sessionViewModel
	done                 chan struct{}
	doneOnce             sync.Once
}

type SessionUpdateHandlerConfig struct {
	Context        context.Context
	Index          int
	AgentID        string
	JobID          string
	SessionID      string
	Logger         *slog.Logger
	RunID          string
	OutWriter      io.Writer
	ErrWriter      io.Writer
	RunJournal     runtimeEventSubmitter
	RunManager     model.RuntimeManager
	JobUsage       *model.Usage
	AggregateUsage *model.Usage
	AggregateMu    *sync.Mutex
	Activity       *activityMonitor
	ReusableAgent  *reusableAgentExecution
}

func NewSessionUpdateHandler(cfg SessionUpdateHandlerConfig) *SessionUpdateHandler {
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}
	if cfg.Logger == nil {
		cfg.Logger = silentLogger()
	}
	return &SessionUpdateHandler{
		ctx:                  cfg.Context,
		index:                cfg.Index,
		agentID:              cfg.AgentID,
		jobID:                cfg.JobID,
		sessionID:            cfg.SessionID,
		logger:               cfg.Logger,
		runID:                cfg.RunID,
		runManager:           cfg.RunManager,
		startedAt:            time.Now(),
		outWriter:            cfg.OutWriter,
		errWriter:            cfg.ErrWriter,
		journal:              cfg.RunJournal,
		jobUsage:             cfg.JobUsage,
		aggregateUsage:       cfg.AggregateUsage,
		aggregateMu:          cfg.AggregateMu,
		activity:             cfg.Activity,
		reusableAgent:        cfg.ReusableAgent,
		blockCounts:          make(map[model.ContentBlockType]int),
		nestedToolCalls:      make(map[string]nestedReusableAgentCall),
		pendingNestedResults: make(map[string]runAgentToolResult),
		sessionView:          newSessionViewModel(),
		done:                 make(chan struct{}),
	}
}

func (h *SessionUpdateHandler) Done() <-chan struct{} {
	return h.done
}

func (h *SessionUpdateHandler) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *SessionUpdateHandler) HandleUpdate(update model.SessionUpdate) error {
	h.recordActivity()

	publicUpdate, err := runtimeevents.PublicSessionUpdate(update)
	if err != nil {
		return fmt.Errorf("convert session update hook payload: %w", err)
	}
	publicUpdateRaw, err := marshalPublicSessionUpdateJSON(publicUpdate)
	if err != nil {
		return fmt.Errorf("marshal public session update: %w", err)
	}
	eventPayload, err := json.Marshal(sessionUpdateEventPayload{
		Index:  h.index,
		Update: publicUpdateRaw,
	})
	if err != nil {
		return fmt.Errorf("marshal session update payload: %w", err)
	}
	model.DispatchObserverHook(
		h.ctx,
		h.runManager,
		"agent.on_session_update",
		sessionUpdateHookPayload{
			RunID:     h.runID,
			JobID:     h.jobID,
			SessionID: h.sessionID,
			Update:    publicUpdateRaw,
		},
	)

	if err := h.renderUpdateBlocks(update.Blocks); err != nil {
		return err
	}
	h.applySessionUpdate(update)
	if err := h.emitReusableAgentLifecycleFromUpdate(update); err != nil {
		h.logger.Warn(
			"failed to emit reusable agent lifecycle from session update; continuing",
			"session_id",
			h.sessionID,
			"update_kind",
			update.Kind,
			"tool_call_id",
			strings.TrimSpace(update.ToolCallID),
			"error",
			err,
		)
	}
	if err := h.emitSessionUpdateEvent(eventPayload); err != nil {
		return err
	}
	if err := h.recordUsageUpdate(update.Usage); err != nil {
		return err
	}

	h.logger.Info(
		"acp session update",
		"agent_id",
		h.agentID,
		"session_id",
		h.sessionID,
		"status",
		update.Status,
		"kind",
		update.Kind,
		"blocks",
		len(update.Blocks),
		"block_types",
		formatBlockTypes(update.Blocks),
		"usage_total",
		update.Usage.Total(),
		"duration",
		time.Since(h.startedAt),
	)
	h.updateCompletionStatus(update.Status)
	return nil
}

func (h *SessionUpdateHandler) renderUpdateBlocks(blocks []model.ContentBlock) error {
	if len(blocks) == 0 {
		return nil
	}

	outLines, errLines := renderContentBlocks(blocks)
	if err := writeRenderedLines(h.outWriter, outLines); err != nil {
		return fmt.Errorf("write ACP session output: %w", err)
	}
	if err := writeRenderedLines(h.errWriter, errLines); err != nil {
		return fmt.Errorf("write ACP session stderr: %w", err)
	}
	h.recordBlockCounts(blocks)
	return nil
}

func (h *SessionUpdateHandler) applySessionUpdate(update model.SessionUpdate) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionView.Apply(update)
}

func (h *SessionUpdateHandler) emitSessionUpdateEvent(payload json.RawMessage) error {
	return h.submitRuntimeEventRaw(events.EventKindSessionUpdate, payload, "session update")
}

func (h *SessionUpdateHandler) recordUsageUpdate(usage model.Usage) error {
	if !hasUsage(usage) {
		return nil
	}
	if h.jobUsage != nil {
		h.jobUsage.Add(usage)
	}
	if err := h.submitRuntimeEvent(
		events.EventKindUsageUpdated,
		runtimeevents.UsagePayload(h.index, usage),
		"usage update",
	); err != nil {
		return err
	}
	if h.aggregateUsage != nil && h.aggregateMu != nil {
		h.aggregateMu.Lock()
		h.aggregateUsage.Add(usage)
		h.aggregateMu.Unlock()
	}
	return nil
}

func (h *SessionUpdateHandler) updateCompletionStatus(status model.SessionStatus) {
	switch status {
	case model.StatusCompleted:
		h.markDone(nil, false)
	case model.StatusFailed:
		h.markDone(fmt.Errorf("ACP session reported failed status"), false)
	}
}

func (h *SessionUpdateHandler) HandleCompletion(err error) error {
	h.recordActivity()
	outcome := agent.SessionOutcome{Status: model.StatusCompleted}
	if err != nil {
		outcome.Status = model.StatusFailed
		outcome.Error = err.Error()
	}
	model.DispatchObserverHook(
		h.ctx,
		h.runManager,
		"agent.post_session_end",
		sessionEndedHookPayload{
			RunID:     h.runID,
			JobID:     h.jobID,
			SessionID: h.sessionID,
			Outcome:   outcome,
		},
	)

	if err != nil {
		if emitErr := h.submitRuntimeEvent(
			events.EventKindSessionFailed,
			kinds.SessionFailedPayload{
				Index: h.index,
				Error: err.Error(),
				Usage: runtimeevents.PublicUsage(sessionHandlerUsage(h.jobUsage)),
			},
			"session failed",
		); emitErr != nil {
			h.markDone(err, true)
			return emitErr
		}
		if writeErr := writeRenderedLines(h.errWriter, []string{"ACP session error: " + err.Error()}); writeErr != nil {
			h.markDone(err, true)
			return fmt.Errorf("write ACP session completion error: %w", writeErr)
		}
		h.logger.Error(
			"acp session error",
			"agent_id",
			h.agentID,
			"session_id",
			h.sessionID,
			"duration",
			time.Since(h.startedAt),
			"error",
			err,
			"block_counts",
			h.snapshotBlockCounts(),
		)
		h.markDone(err, true)
		return nil
	}
	if err := h.submitRuntimeEvent(
		events.EventKindSessionCompleted,
		kinds.SessionCompletedPayload{
			Index: h.index,
			Usage: runtimeevents.PublicUsage(sessionHandlerUsage(h.jobUsage)),
		},
		"session completed",
	); err != nil {
		h.markDone(nil, false)
		return err
	}

	h.logger.Info(
		"acp session completed",
		"agent_id",
		h.agentID,
		"session_id",
		h.sessionID,
		"duration",
		time.Since(h.startedAt),
		"block_counts",
		h.snapshotBlockCounts(),
	)
	h.markDone(nil, false)
	return nil
}

func (h *SessionUpdateHandler) submitRuntimeEvent(
	kind events.EventKind,
	payload any,
	description string,
) error {
	if !hasRuntimeEventSubmitter(h.journal) {
		return nil
	}

	event, err := runtimeevents.NewRuntimeEvent(h.runID, kind, payload)
	if err != nil {
		return err
	}
	return h.submitEncodedRuntimeEvent(event, description)
}

func (h *SessionUpdateHandler) submitRuntimeEventRaw(
	kind events.EventKind,
	payload json.RawMessage,
	description string,
) error {
	if !hasRuntimeEventSubmitter(h.journal) {
		return nil
	}

	event := runtimeevents.NewRuntimeEventRaw(h.runID, kind, payload)
	return h.submitEncodedRuntimeEvent(event, description)
}

func (h *SessionUpdateHandler) submitEncodedRuntimeEvent(event events.Event, description string) error {
	if err := h.journal.Submit(h.ctx, event); err != nil {
		return fmt.Errorf("submit %s event: %w", description, err)
	}
	return nil
}

func (h *SessionUpdateHandler) recordActivity() {
	if h.activity != nil {
		h.activity.RecordActivity()
	}
}

func (h *SessionUpdateHandler) recordBlockCounts(blocks []model.ContentBlock) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, block := range blocks {
		h.blockCounts[block.Type]++
	}
}

func (h *SessionUpdateHandler) snapshotBlockCounts() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.blockCounts) == 0 {
		return ""
	}

	keys := make([]string, 0, len(h.blockCounts))
	for blockType, count := range h.blockCounts {
		keys = append(keys, fmt.Sprintf("%s=%d", blockType, count))
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func (h *SessionUpdateHandler) markDone(err error, override bool) {
	h.mu.Lock()
	if err != nil && (override || h.err == nil) {
		h.err = err
	}
	h.mu.Unlock()

	h.doneOnce.Do(func() {
		close(h.done)
	})
}

func (h *SessionUpdateHandler) Snapshot() SessionViewSnapshot {
	if h == nil {
		return SessionViewSnapshot{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessionView.Snapshot()
}

func hasUsage(usage model.Usage) bool {
	return usage.InputTokens != 0 ||
		usage.OutputTokens != 0 ||
		usage.TotalTokens != 0 ||
		usage.CacheReads != 0 ||
		usage.CacheWrites != 0
}

func sessionHandlerUsage(usage *model.Usage) model.Usage {
	if usage == nil {
		return model.Usage{}
	}
	return *usage
}

func formatBlockTypes(blocks []model.ContentBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	counts := make(map[model.ContentBlockType]int)
	for _, block := range blocks {
		counts[block.Type]++
	}
	keys := make([]string, 0, len(counts))
	for blockType, count := range counts {
		keys = append(keys, fmt.Sprintf("%s=%d", blockType, count))
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

type sessionUpdateHookPayload struct {
	RunID     string          `json:"run_id"`
	JobID     string          `json:"job_id"`
	SessionID string          `json:"session_id"`
	Update    json.RawMessage `json:"update"`
}

type sessionUpdateEventPayload struct {
	Index  int             `json:"index"`
	Update json.RawMessage `json:"update"`
}

type sessionEndedHookPayload struct {
	RunID     string               `json:"run_id"`
	JobID     string               `json:"job_id"`
	SessionID string               `json:"session_id"`
	Outcome   agent.SessionOutcome `json:"outcome"`
}
