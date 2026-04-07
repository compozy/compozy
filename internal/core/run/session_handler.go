package run

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type sessionUpdateHandler struct {
	ctx            context.Context
	index          int
	agentID        string
	sessionID      string
	logger         *slog.Logger
	runID          string
	startedAt      time.Time
	outWriter      io.Writer
	errWriter      io.Writer
	journal        *journal.Journal
	jobUsage       *model.Usage
	aggregateUsage *model.Usage
	aggregateMu    *sync.Mutex
	activity       *activityMonitor

	mu          sync.Mutex
	err         error
	blockCounts map[model.ContentBlockType]int
	sessionView *sessionViewModel
	done        chan struct{}
	doneOnce    sync.Once
}

func newSessionUpdateHandler(
	ctx context.Context,
	index int,
	agentID string,
	sessionID string,
	logger *slog.Logger,
	runID string,
	outWriter io.Writer,
	errWriter io.Writer,
	runJournal *journal.Journal,
	jobUsage *model.Usage,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	activity *activityMonitor,
) *sessionUpdateHandler {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = silentLogger()
	}
	return &sessionUpdateHandler{
		ctx:            ctx,
		index:          index,
		agentID:        agentID,
		sessionID:      sessionID,
		logger:         logger,
		runID:          runID,
		startedAt:      time.Now(),
		outWriter:      outWriter,
		errWriter:      errWriter,
		journal:        runJournal,
		jobUsage:       jobUsage,
		aggregateUsage: aggregateUsage,
		aggregateMu:    aggregateMu,
		activity:       activity,
		blockCounts:    make(map[model.ContentBlockType]int),
		sessionView:    newSessionViewModel(),
		done:           make(chan struct{}),
	}
}

func (h *sessionUpdateHandler) Done() <-chan struct{} {
	return h.done
}

func (h *sessionUpdateHandler) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *sessionUpdateHandler) HandleUpdate(update model.SessionUpdate) error {
	h.recordActivity()

	if err := h.renderUpdateBlocks(update.Blocks); err != nil {
		return err
	}
	h.applySessionUpdate(update)
	if err := h.emitSessionUpdateEvent(update); err != nil {
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

func (h *sessionUpdateHandler) renderUpdateBlocks(blocks []model.ContentBlock) error {
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

func (h *sessionUpdateHandler) applySessionUpdate(update model.SessionUpdate) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionView.Apply(update)
}

func (h *sessionUpdateHandler) emitSessionUpdateEvent(update model.SessionUpdate) error {
	publicUpdate, err := publicSessionUpdate(update)
	if err != nil {
		return fmt.Errorf("convert session update event payload: %w", err)
	}

	return h.submitRuntimeEvent(
		events.EventKindSessionUpdate,
		kinds.SessionUpdatePayload{
			Index:  h.index,
			Update: publicUpdate,
		},
		"session update",
	)
}

func (h *sessionUpdateHandler) recordUsageUpdate(usage model.Usage) error {
	if !hasUsage(usage) {
		return nil
	}
	if h.jobUsage != nil {
		h.jobUsage.Add(usage)
	}
	if err := h.submitRuntimeEvent(
		events.EventKindUsageUpdated,
		usagePayload(h.index, usage),
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

func (h *sessionUpdateHandler) updateCompletionStatus(status model.SessionStatus) {
	switch status {
	case model.StatusCompleted:
		h.markDone(nil, false)
	case model.StatusFailed:
		h.markDone(fmt.Errorf("ACP session reported failed status"), false)
	}
}

func (h *sessionUpdateHandler) HandleCompletion(err error) error {
	h.recordActivity()

	if err != nil {
		if emitErr := h.submitRuntimeEvent(
			events.EventKindSessionFailed,
			kinds.SessionFailedPayload{
				Index: h.index,
				Error: err.Error(),
				Usage: publicUsage(sessionHandlerUsage(h.jobUsage)),
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
			Usage: publicUsage(sessionHandlerUsage(h.jobUsage)),
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

func (h *sessionUpdateHandler) submitRuntimeEvent(
	kind events.EventKind,
	payload any,
	description string,
) error {
	if h.journal == nil {
		return nil
	}

	event, err := newRuntimeEvent(h.runID, kind, payload)
	if err != nil {
		return err
	}
	if err := h.journal.Submit(h.ctx, event); err != nil {
		return fmt.Errorf("submit %s event: %w", description, err)
	}
	return nil
}

func (h *sessionUpdateHandler) recordActivity() {
	if h.activity != nil {
		h.activity.recordActivity()
	}
}

func (h *sessionUpdateHandler) recordBlockCounts(blocks []model.ContentBlock) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, block := range blocks {
		h.blockCounts[block.Type]++
	}
}

func (h *sessionUpdateHandler) snapshotBlockCounts() string {
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

func (h *sessionUpdateHandler) markDone(err error, override bool) {
	h.mu.Lock()
	if err != nil && (override || h.err == nil) {
		h.err = err
	}
	h.mu.Unlock()

	h.doneOnce.Do(func() {
		close(h.done)
	})
}

func (h *sessionUpdateHandler) Snapshot() SessionViewSnapshot {
	if h == nil {
		return SessionViewSnapshot{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessionView.snapshot()
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

func cloneContentBlocks(blocks []model.ContentBlock) []model.ContentBlock {
	if len(blocks) == 0 {
		return nil
	}

	cloned := make([]model.ContentBlock, len(blocks))
	for i, block := range blocks {
		cloned[i] = model.ContentBlock{
			Type: block.Type,
			Data: append([]byte(nil), block.Data...),
		}
	}
	return cloned
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
