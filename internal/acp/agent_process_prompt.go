package acp

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
)

type activePromptState struct {
	turnID     string
	runID      string
	generation int64
	events     chan AgentEvent
	activity   chan struct{}
	detached   chan struct{}
	cancel     context.CancelFunc

	sendMu sync.Mutex
	closed bool

	seenToolCalls        map[string]toolCallProjection
	pendingToolResults   []AgentEvent
	pendingToolResultIDs map[string]struct{}

	usageMu sync.Mutex
	usage   TokenUsage
}

type toolCallProjection struct {
	title       string
	name        string
	kind        string
	inputDigest [sha256.Size]byte
	hasInput    bool
	prechecked  bool
}

const maxPendingToolResults = 128

func (p *AgentProcess) beginPrompt(
	turnID string,
	bufferSize int,
	cancelFns ...context.CancelFunc,
) (*activePromptState, error) {
	return p.beginPromptState(turnID, "", 0, bufferSize, cancelFns...)
}

func (p *AgentProcess) beginPromptForRun(
	turnID string,
	runID string,
	generation int64,
	bufferSize int,
	cancelFns ...context.CancelFunc,
) (*activePromptState, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" || generation <= 0 {
		return nil, errors.New("acp: prompt run identity is incomplete")
	}
	return p.beginPromptState(turnID, runID, generation, bufferSize, cancelFns...)
}

func (p *AgentProcess) beginPromptState(
	turnID string,
	runID string,
	generation int64,
	bufferSize int,
	cancelFns ...context.CancelFunc,
) (*activePromptState, error) {
	p.promptMu.Lock()
	defer p.promptMu.Unlock()
	if p.activePrompt != nil {
		return nil, errors.New("acp: prompt already in progress")
	}
	if bufferSize <= 0 {
		bufferSize = 1
	}
	var cancel context.CancelFunc
	if len(cancelFns) > 0 {
		cancel = cancelFns[0]
	}
	active := &activePromptState{
		turnID:               turnID,
		runID:                runID,
		generation:           generation,
		events:               make(chan AgentEvent, bufferSize),
		activity:             make(chan struct{}, 1),
		detached:             make(chan struct{}),
		cancel:               cancel,
		seenToolCalls:        make(map[string]toolCallProjection),
		pendingToolResultIDs: make(map[string]struct{}),
	}
	p.activePrompt = active
	return active, nil
}

func (p *AgentProcess) endPrompt(active *activePromptState) {
	if active == nil {
		return
	}

	detached := false
	p.promptMu.Lock()
	if p.activePrompt == active {
		p.activePrompt = nil
		detached = true
	}
	p.promptMu.Unlock()
	if detached {
		close(active.detached)
	}

	active.sendMu.Lock()
	defer active.sendMu.Unlock()
	if !active.closed {
		active.closed = true
		close(active.events)
	}
}

func (p *AgentProcess) currentPrompt() *activePromptState {
	p.promptMu.RLock()
	defer p.promptMu.RUnlock()
	return p.activePrompt
}

func (p *AgentProcess) cancelCurrentPrompt() bool {
	p.promptMu.RLock()
	active := p.activePrompt
	p.promptMu.RUnlock()
	if active == nil || active.cancel == nil {
		return false
	}
	active.cancel()
	return true
}

// SetTurnSourceProvider configures a daemon-local callback that reports the current turn provenance.
func (p *AgentProcess) SetTurnSourceProvider(provider func() string) {
	if p == nil {
		return
	}

	p.turnSourceProviderMu.Lock()
	defer p.turnSourceProviderMu.Unlock()
	p.turnSourceProvider = provider
}

func (p *AgentProcess) currentTurnSource() string {
	if p == nil {
		return ""
	}

	p.turnSourceProviderMu.RLock()
	provider := p.turnSourceProvider
	p.turnSourceProviderMu.RUnlock()
	if provider == nil {
		return ""
	}
	return strings.TrimSpace(provider())
}

func (p *AgentProcess) isNetworkTurn() bool {
	return p.currentTurnSource() == networkCommandName
}

func (p *AgentProcess) nextPromptText(message string) (string, bool, SystemPromptDeliveryMode) {
	userMessage := strings.TrimSpace(message)

	p.systemPromptMu.Lock()
	defer p.systemPromptMu.Unlock()

	systemPrompt := strings.TrimSpace(p.systemPrompt)
	if p.systemPromptSent || systemPrompt == "" {
		return userMessage, false, ""
	}

	delivery := p.systemPromptDelivery
	if delivery == "" {
		delivery = SystemPromptDeliveryFirstTurnPrefix
	}
	if delivery == SystemPromptDeliveryNative {
		return userMessage, true, delivery
	}
	return strings.TrimSpace(
		"Session instructions (treat as system guidance for this conversation):\n\n" +
			systemPrompt +
			"\n\nUser request:\n\n" +
			userMessage,
	), true, delivery
}

func (p *AgentProcess) markSystemPromptSent() {
	p.systemPromptMu.Lock()
	defer p.systemPromptMu.Unlock()
	p.systemPromptSent = true
}

func (p *AgentProcess) emitPromptEvent(event AgentEvent) {
	p.promptMu.RLock()
	active := p.activePrompt
	p.promptMu.RUnlock()
	if active == nil {
		return
	}

	active.sendMu.Lock()
	defer active.sendMu.Unlock()
	if active.closed {
		return
	}
	if active.deferToolResultLocked(event) {
		return
	}
	if active.shouldSuppressToolCallLocked(event) {
		return
	}
	if event.Type == EventTypeDone {
		active.flushDeferredToolResultsLocked()
	}
	active.sendEventLocked(event)
	if event.Type == EventTypeToolCall {
		active.flushDeferredToolResultsForToolLocked(event.ToolCallID)
	}
}

func (a *activePromptState) deferToolResultLocked(event AgentEvent) bool {
	if a == nil || event.Type != EventTypeToolResult {
		return false
	}
	toolCallID := strings.TrimSpace(event.ToolCallID)
	if toolCallID == "" {
		return false
	}
	if _, ok := a.seenToolCalls[toolCallID]; ok {
		return false
	}
	if _, ok := a.pendingToolResultIDs[toolCallID]; ok {
		return true
	}
	if len(a.pendingToolResults) >= maxPendingToolResults {
		a.dropOldestPendingToolResultLocked()
	}
	a.pendingToolResults = append(a.pendingToolResults, event)
	a.pendingToolResultIDs[toolCallID] = struct{}{}
	return true
}

func (a *activePromptState) shouldSuppressToolCallLocked(event AgentEvent) bool {
	if a == nil || event.Type != EventTypeToolCall {
		return false
	}
	toolCallID := strings.TrimSpace(event.ToolCallID)
	if toolCallID == "" {
		return false
	}

	projection := newToolCallProjection(event)
	current, ok := a.seenToolCalls[toolCallID]
	if !ok {
		a.seenToolCalls[toolCallID] = projection
		return false
	}

	merged, changed := current.merge(projection)
	a.seenToolCalls[toolCallID] = merged
	return !changed
}

func newToolCallProjection(event AgentEvent) toolCallProjection {
	projection := toolCallProjection{
		title:      strings.TrimSpace(event.Title),
		name:       strings.TrimSpace(event.ToolName()),
		kind:       strings.TrimSpace(event.ToolKind()),
		prechecked: event.ToolPrechecked(),
	}
	input := event.ToolInput()
	if len(input) > 0 {
		projection.inputDigest = sha256.Sum256(input)
		projection.hasInput = true
	}
	return projection
}

func (p toolCallProjection) merge(next toolCallProjection) (toolCallProjection, bool) {
	changed := false
	if next.title != "" && next.title != p.title {
		p.title = next.title
		changed = true
	}
	if next.name != "" && next.name != p.name {
		p.name = next.name
		changed = true
	}
	if next.kind != "" && next.kind != p.kind {
		p.kind = next.kind
		changed = true
	}
	if next.hasInput && (!p.hasInput || next.inputDigest != p.inputDigest) {
		p.inputDigest = next.inputDigest
		p.hasInput = true
		changed = true
	}
	if next.prechecked && !p.prechecked {
		p.prechecked = true
		changed = true
	}
	return p, changed
}

func (a *activePromptState) flushDeferredToolResultsForToolLocked(toolCallID string) {
	if a == nil {
		return
	}
	trimmed := strings.TrimSpace(toolCallID)
	if trimmed == "" || len(a.pendingToolResults) == 0 {
		return
	}

	remaining := a.pendingToolResults[:0]
	remainingIDs := make(map[string]struct{}, len(a.pendingToolResultIDs))
	for _, event := range a.pendingToolResults {
		eventToolCallID := strings.TrimSpace(event.ToolCallID)
		if eventToolCallID == trimmed {
			a.sendEventLocked(event)
			continue
		}
		remaining = append(remaining, event)
		if eventToolCallID != "" {
			remainingIDs[eventToolCallID] = struct{}{}
		}
	}
	a.pendingToolResults = remaining
	a.pendingToolResultIDs = remainingIDs
}

func (a *activePromptState) flushDeferredToolResultsLocked() {
	if a == nil || len(a.pendingToolResults) == 0 {
		return
	}
	pending := a.pendingToolResults
	a.pendingToolResults = nil
	a.pendingToolResultIDs = make(map[string]struct{})
	for _, event := range pending {
		a.sendEventLocked(event)
	}
}

func (a *activePromptState) dropOldestPendingToolResultLocked() {
	if a == nil || len(a.pendingToolResults) == 0 {
		return
	}
	oldest := a.pendingToolResults[0]
	delete(a.pendingToolResultIDs, strings.TrimSpace(oldest.ToolCallID))
	a.pendingToolResults = append(a.pendingToolResults[:0], a.pendingToolResults[1:]...)
}

func (a *activePromptState) sendEventLocked(event AgentEvent) {
	if a == nil {
		return
	}
	a.events <- event
	select {
	case a.activity <- struct{}{}:
	default:
	}
}

func (p *AgentProcess) mergePromptUsage(update TokenUsage) TokenUsage {
	active := p.currentPrompt()
	if active == nil {
		return update
	}
	active.usageMu.Lock()
	defer active.usageMu.Unlock()
	if active.usage.IsZero() {
		active.usage = update
		return active.usage
	}
	active.usage = active.usage.Merge(update)
	return active.usage
}
