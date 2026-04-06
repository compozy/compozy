package run

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
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
	if logger == nil {
		logger = silentLogger()
	}
	return &sessionUpdateHandler{
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
	if err := h.journal.Submit(context.Background(), event); err != nil {
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

func writeRenderedLines(dst io.Writer, lines []string) error {
	if dst == nil || len(lines) == 0 {
		return nil
	}

	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	_, err := io.WriteString(dst, builder.String())
	return err
}

func renderContentBlocks(blocks []model.ContentBlock) ([]string, []string) {
	var outLines []string
	var errLines []string
	for _, block := range blocks {
		renderedOut, renderedErr := renderContentBlock(block)
		outLines = append(outLines, renderedOut...)
		errLines = append(errLines, renderedErr...)
	}
	return outLines, errLines
}

func renderContentBlock(block model.ContentBlock) ([]string, []string) {
	switch block.Type {
	case model.BlockText:
		return renderTextBlock(block)
	case model.BlockToolUse:
		return renderToolUseBlock(block)
	case model.BlockToolResult:
		return renderToolResultBlock(block)
	case model.BlockDiff:
		return renderDiffBlock(block)
	case model.BlockTerminalOutput:
		return renderTerminalOutputBlock(block)
	case model.BlockImage:
		return renderImageBlock(block)
	default:
		return []string{strings.TrimSpace(string(block.Data))}, nil
	}
}

func renderTextBlock(block model.ContentBlock) ([]string, []string) {
	textBlock, err := block.AsText()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}
	return splitRenderedText(textBlock.Text), nil
}

func renderToolUseBlock(block model.ContentBlock) ([]string, []string) {
	toolUse, err := block.AsToolUse()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	line := fmt.Sprintf("[TOOL] %s (%s)", toolUseDisplayTitle(toolUse), toolUse.ID)
	outLines := []string{line}
	payload := toolUse.Input
	if len(payload) == 0 {
		payload = toolUse.RawInput
	}
	if len(payload) > 0 {
		outLines = append(outLines, splitRenderedText(string(payload))...)
	}
	return outLines, nil
}

func renderToolResultBlock(block model.ContentBlock) ([]string, []string) {
	toolResult, err := block.AsToolResult()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	lines := splitRenderedText(toolResult.Content)
	if len(lines) == 0 {
		lines = []string{fmt.Sprintf("[TOOL RESULT] %s", toolResult.ToolUseID)}
	}
	if toolResult.IsError {
		return nil, lines
	}
	return lines, nil
}

func renderDiffBlock(block model.ContentBlock) ([]string, []string) {
	diffBlock, err := block.AsDiff()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}
	return splitRenderedText(diffBlock.Diff), nil
}

func renderTerminalOutputBlock(block model.ContentBlock) ([]string, []string) {
	terminalBlock, err := block.AsTerminalOutput()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	lines := make([]string, 0, 4)
	if terminalBlock.Command != "" {
		lines = append(lines, "$ "+terminalBlock.Command)
	}
	lines = append(lines, splitRenderedText(terminalBlock.Output)...)
	if terminalBlock.ExitCode != 0 {
		lines = append(lines, fmt.Sprintf("[exit code: %d]", terminalBlock.ExitCode))
	}
	return lines, nil
}

func renderImageBlock(block model.ContentBlock) ([]string, []string) {
	imageBlock, err := block.AsImage()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	location := "inline"
	if imageBlock.URI != nil && *imageBlock.URI != "" {
		location = *imageBlock.URI
	}
	return []string{fmt.Sprintf("[IMAGE] %s %s", imageBlock.MimeType, location)}, nil
}

func renderDecodeFailure(block model.ContentBlock, err error) []string {
	payload := strings.TrimSpace(string(block.Data))
	if payload == "" {
		payload = fmt.Sprintf("type=%s", block.Type)
	}
	return []string{fmt.Sprintf("[decode %s block failed] %v", block.Type, err), payload}
}

func splitRenderedText(text string) []string {
	if text == "" {
		return nil
	}

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}

type lineBuffer struct {
	mu    sync.Mutex
	capN  int
	lines []string
}

func newLineBuffer(n int) *lineBuffer {
	if n < 0 {
		n = 0
	}
	initialCap := n
	if initialCap <= 0 {
		initialCap = 32
	}
	return &lineBuffer{capN: n, lines: make([]string, 0, initialCap)}
}

func (r *lineBuffer) appendLine(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, s)
	if r.capN > 0 && len(r.lines) > r.capN {
		r.lines = r.lines[len(r.lines)-r.capN:]
	}
}

func (r *lineBuffer) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

type activityMonitor struct {
	mu           sync.Mutex
	lastActivity time.Time
}

func newActivityMonitor() *activityMonitor {
	return &activityMonitor{lastActivity: time.Now()}
}

func (a *activityMonitor) recordActivity() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = time.Now()
}

func (a *activityMonitor) timeSinceLastActivity() time.Duration {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.lastActivity)
}

func appendLinesToBuffer(buf *lineBuffer, lines []string) {
	if buf == nil {
		return
	}
	for _, line := range lines {
		buf.appendLine(line)
	}
}

func createLogWriters(outFile *os.File, errFile *os.File, useUI bool, emitHuman bool) (io.Writer, io.Writer) {
	if useUI || !emitHuman {
		return writerOrNil(outFile), writerOrNil(errFile)
	}
	return io.MultiWriter(writerOrNil(outFile), os.Stdout), io.MultiWriter(writerOrNil(errFile), os.Stderr)
}

func writerOrNil(file *os.File) io.Writer {
	if file == nil {
		return nil
	}
	return file
}
