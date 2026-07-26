package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
)

// SyntheticPromptOpts carries daemon-owned synthetic prompt input plus
// wake-up metadata required for persistence and later reentry handling.
type SyntheticPromptOpts struct {
	Message                 string
	Metadata                acp.PromptSyntheticMeta
	TurnID                  string
	SkipIfBusy              bool
	InterruptIfAgentWaiting bool
}

type queuedSyntheticPrompt struct {
	ctx     context.Context
	request promptRequest
	out     chan acp.AgentEvent
}

// PromptSynthetic submits one daemon-owned synthetic prompt turn.
func (m *Manager) PromptSynthetic(
	ctx context.Context,
	id string,
	opts SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	req, err := m.parseSyntheticPromptRequest(ctx, id, opts)
	if err != nil {
		return nil, err
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return nil, err
	}

	session, err := m.lookupPromptSession(ctx, req.target)
	if err != nil {
		return nil, err
	}

	dispatchCtx := context.WithoutCancel(ctx)
	if session.IsPrompting() || m.hasQueuedSyntheticPrompt(req.target) {
		if opts.InterruptIfAgentWaiting &&
			!opts.SkipIfBusy &&
			!m.hasQueuedSyntheticPrompt(req.target) &&
			session.isCurrentPromptAgentWaiting() {
			eventsCh, interruptErr := m.interruptAndSubmitSyntheticPrompt(dispatchCtx, session, req)
			if interruptErr == nil {
				return eventsCh, nil
			}
			if !isRetryableSyntheticInterruptError(interruptErr) {
				return nil, interruptErr
			}
		}
		if opts.SkipIfBusy {
			return nil, ErrPromptInProgress
		}
		return m.enqueueSyntheticPrompt(dispatchCtx, req), nil
	}

	eventsCh, err := m.submitPromptRequest(dispatchCtx, req)
	if err == nil {
		return eventsCh, nil
	}
	if !errors.Is(err, ErrPromptInProgress) {
		return nil, err
	}
	if opts.SkipIfBusy {
		return nil, ErrPromptInProgress
	}

	return m.enqueueSyntheticPrompt(dispatchCtx, req), nil
}

func (m *Manager) interruptAndSubmitSyntheticPrompt(
	ctx context.Context,
	session *Session,
	req promptRequest,
) (<-chan acp.AgentEvent, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if session == nil {
		return nil, errors.New("session: session is required")
	}
	interruptCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.supervision.TimeoutCancelGrace)
	defer cancel()
	if err := m.CancelPrompt(interruptCtx, session.ID); err != nil {
		return nil, err
	}
	if err := waitForPromptIdle(interruptCtx, session); err != nil {
		return nil, err
	}
	return m.submitPromptRequest(ctx, req)
}

func isRetryableSyntheticInterruptError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrPromptInProgress) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

func (m *Manager) parseSyntheticPromptRequest(
	ctx context.Context,
	id string,
	opts SyntheticPromptOpts,
) (promptRequest, error) {
	if ctx == nil {
		return promptRequest{}, errors.New("session: prompt context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return promptRequest{}, errors.New("session: session id is required")
	}

	message := strings.TrimSpace(opts.Message)
	if message == "" {
		return promptRequest{}, errors.New("session: prompt message is required")
	}

	meta, err := normalizePromptMeta(
		TurnSourceSynthetic,
		acp.PromptMeta{
			TurnSource: acp.PromptTurnSourceSynthetic,
			Synthetic:  &opts.Metadata,
		},
		promptSubmissionPathSynthetic,
	)
	if err != nil {
		return promptRequest{}, err
	}

	turnID := strings.TrimSpace(opts.TurnID)
	if turnID == "" {
		turnID = m.newPromptTurnID()
	}

	return promptRequest{
		turnID:          turnID,
		target:          target,
		message:         message,
		authoredMessage: message,
		turnSource:      TurnSourceSynthetic,
		meta:            meta,
	}, nil
}

func (m *Manager) enqueueSyntheticPrompt(ctx context.Context, req promptRequest) <-chan acp.AgentEvent {
	bufferSize := 1
	if m != nil && m.promptBufSize > 0 {
		bufferSize = m.promptBufSize
	}

	item := queuedSyntheticPrompt{
		ctx:     ctx,
		request: req,
		out:     make(chan acp.AgentEvent, bufferSize),
	}

	m.syntheticMu.Lock()
	m.syntheticQueues[req.target] = append(m.syntheticQueues[req.target], item)
	m.syntheticMu.Unlock()

	m.startNextQueuedSyntheticPrompt(req.target)
	return item.out
}

func (m *Manager) hasQueuedSyntheticPrompt(sessionID string) bool {
	if m == nil {
		return false
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return false
	}

	m.syntheticMu.Lock()
	defer m.syntheticMu.Unlock()
	return len(m.syntheticQueues[target]) > 0
}

func (m *Manager) startNextQueuedSyntheticPrompt(sessionID string) {
	if m == nil {
		return
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return
	}

	item, ok := m.claimQueuedSyntheticPrompt(target)
	if !ok {
		return
	}

	session, err := m.lookupPromptSession(item.ctx, target)
	if err != nil {
		failed := m.finishQueuedSyntheticDispatchAndDrain(target)
		m.emitQueuedSyntheticDispatchError(item, err)
		for _, queued := range failed {
			m.emitQueuedSyntheticDispatchError(queued, err)
		}
		return
	}
	if session.IsPrompting() {
		m.requeueSyntheticPromptFrontAndFinishDispatch(target, item)
		return
	}

	source, err := m.submitPromptRequest(item.ctx, item.request)
	if err != nil {
		if errors.Is(err, ErrPromptInProgress) {
			m.requeueSyntheticPromptFrontAndFinishDispatch(target, item)
			return
		}
		m.finishQueuedSyntheticDispatch(target)
		m.emitQueuedSyntheticDispatchError(item, err)
		m.startNextQueuedSyntheticPrompt(target)
		return
	}

	go m.forwardQueuedSyntheticPrompt(target, item.out, source)
}

func (m *Manager) claimQueuedSyntheticPrompt(sessionID string) (queuedSyntheticPrompt, bool) {
	if m == nil {
		return queuedSyntheticPrompt{}, false
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return queuedSyntheticPrompt{}, false
	}

	m.syntheticMu.Lock()
	defer m.syntheticMu.Unlock()

	if m.syntheticDispatching[target] {
		return queuedSyntheticPrompt{}, false
	}

	queue := m.syntheticQueues[target]
	if len(queue) == 0 {
		return queuedSyntheticPrompt{}, false
	}

	item := queue[0]
	if len(queue) == 1 {
		delete(m.syntheticQueues, target)
	} else {
		m.syntheticQueues[target] = append([]queuedSyntheticPrompt(nil), queue[1:]...)
	}
	m.syntheticDispatching[target] = true
	return item, true
}

func (m *Manager) finishQueuedSyntheticDispatch(sessionID string) {
	if m == nil {
		return
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return
	}

	m.syntheticMu.Lock()
	defer m.syntheticMu.Unlock()
	m.finishQueuedSyntheticDispatchLocked(target)
}

func (m *Manager) requeueSyntheticPromptFrontAndFinishDispatch(sessionID string, item queuedSyntheticPrompt) {
	if m == nil {
		return
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return
	}

	m.syntheticMu.Lock()
	defer m.syntheticMu.Unlock()

	m.finishQueuedSyntheticDispatchLocked(target)
	queue := m.syntheticQueues[target]
	next := make([]queuedSyntheticPrompt, 0, len(queue)+1)
	next = append(next, item)
	next = append(next, queue...)
	m.syntheticQueues[target] = next
}

func (m *Manager) finishQueuedSyntheticDispatchAndDrain(sessionID string) []queuedSyntheticPrompt {
	if m == nil {
		return nil
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil
	}

	m.syntheticMu.Lock()
	defer m.syntheticMu.Unlock()

	m.finishQueuedSyntheticDispatchLocked(target)
	queue := append([]queuedSyntheticPrompt(nil), m.syntheticQueues[target]...)
	delete(m.syntheticQueues, target)
	return queue
}

func (m *Manager) finishQueuedSyntheticDispatchLocked(sessionID string) {
	delete(m.syntheticDispatching, sessionID)
}

func (m *Manager) forwardQueuedSyntheticPrompt(
	sessionID string,
	out chan<- acp.AgentEvent,
	source <-chan acp.AgentEvent,
) {
	defer close(out)
	defer func() {
		m.finishQueuedSyntheticDispatch(sessionID)
		m.startNextQueuedSyntheticPrompt(sessionID)
	}()

	for event := range source {
		out <- event
	}
}

func (m *Manager) failQueuedSyntheticPrompts(sessionID string, err error) {
	if m == nil {
		return
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return
	}

	m.syntheticMu.Lock()
	queue := append([]queuedSyntheticPrompt(nil), m.syntheticQueues[target]...)
	delete(m.syntheticQueues, target)
	m.syntheticMu.Unlock()

	for _, item := range queue {
		m.emitQueuedSyntheticDispatchError(item, err)
	}
}

func (m *Manager) emitQueuedSyntheticDispatchError(item queuedSyntheticPrompt, err error) {
	defer close(item.out)

	if err == nil {
		return
	}

	summary := fmt.Sprintf("session: synthetic prompt dropped for %q: %v", item.request.target, err)
	if m != nil && m.logger != nil {
		m.logger.Warn(summary, "session_id", item.request.target, "turn_id", item.request.turnID)
	}

	timestamp := time.Now().UTC()
	if m != nil && m.now != nil {
		timestamp = m.now()
	}

	item.out <- acp.AgentEvent{
		Type:      acp.EventTypeError,
		TurnID:    item.request.turnID,
		Timestamp: timestamp,
		Error:     summary,
	}
}
