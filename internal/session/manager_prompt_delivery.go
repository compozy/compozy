package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	promptDeliveryCatchUpInterval = 25 * time.Millisecond
	promptDeliveryPageSize        = 200
	promptDeliveryReadAttempts    = 3
)

type durablePromptDelivery struct {
	events          <-chan acp.AgentEvent
	persistenceDone chan struct{}
	cancel          context.CancelFunc
}

// openDurablePromptDelivery anchors live delivery before the provider can append this turn's output.
func (m *Manager) openDurablePromptDelivery(
	ctx context.Context,
	session *Session,
	turnID string,
) (durablePromptDelivery, error) {
	recorder := session.recorderHandle()
	if recorder == nil {
		return durablePromptDelivery{}, errors.New("session: event recorder is not available")
	}
	afterSequence, err := recorder.MaxEventSequence(ctx)
	if err != nil {
		return durablePromptDelivery{}, err
	}
	deliveryCtx, cancel := context.WithCancel(ctx)
	persisted, unsubscribe, err := m.subscribePersistedSessionEvents(deliveryCtx, session.ID, afterSequence)
	if err != nil {
		cancel()
		return durablePromptDelivery{}, err
	}

	out := make(chan acp.AgentEvent, m.promptBufSize)
	persistenceDone := make(chan struct{})
	m.startTrackedPromptTask(func() {
		m.runDurablePromptDelivery(
			deliveryCtx,
			session,
			turnID,
			afterSequence,
			persisted,
			unsubscribe,
			persistenceDone,
			out,
		)
	})
	return durablePromptDelivery{events: out, persistenceDone: persistenceDone, cancel: cancel}, nil
}

// runDurablePromptDelivery keeps a contiguous cursor so reordered publications cannot hide terminal events.
func (m *Manager) runDurablePromptDelivery(
	ctx context.Context,
	session *Session,
	turnID string,
	afterSequence int64,
	live <-chan store.SessionEvent,
	unsubscribe func(),
	persistenceDone <-chan struct{},
	out chan<- acp.AgentEvent,
) {
	defer close(out)
	defer unsubscribe()

	persistenceComplete := false
	// One turn's events resolve their session-message cards in order, like its transcript entry.
	cards := &transcript.SessionMessageCards{}
	for live != nil {
		select {
		case <-ctx.Done():
			return
		case <-persistenceDone:
			persistenceDone = nil
			persistenceComplete = true
			live = nil
		case persisted, ok := <-live:
			if !ok {
				live = nil
				continue
			}
			// Concurrent publishers can overtake an earlier append; replay before advancing past a gap.
			if persisted.Sequence > afterSequence+1 {
				var finished bool
				afterSequence, finished = m.catchUpDurablePromptDelivery(
					ctx, session, turnID, afterSequence, persisted.Sequence, cards, persistenceDone, false, out,
				)
				if finished {
					return
				}
				continue
			}
			if persisted.Sequence <= afterSequence {
				continue
			}
			terminal, delivered := m.deliverPersistedPromptEvent(
				ctx, session, turnID, persisted, cards, persistenceDone, out,
			)
			if persisted.Sequence > afterSequence {
				afterSequence = persisted.Sequence
			}
			if delivered && terminal {
				return
			}
		}
	}

	m.catchUpDurablePromptDelivery(
		ctx,
		session,
		turnID,
		afterSequence,
		0,
		cards,
		persistenceDone,
		persistenceComplete,
		out,
	)
}

// catchUpDurablePromptDelivery replays through a live watermark, or awaits a terminal when it is zero.
func (m *Manager) catchUpDurablePromptDelivery(
	ctx context.Context,
	session *Session,
	turnID string,
	afterSequence, throughSequence int64,
	cards *transcript.SessionMessageCards,
	persistenceDone <-chan struct{},
	persistenceComplete bool,
	out chan<- acp.AgentEvent,
) (int64, bool) {
	ticker := time.NewTicker(promptDeliveryCatchUpInterval)
	defer ticker.Stop()
	recorder := session.recorderHandle()
	if recorder == nil {
		m.deliverPromptProjectionFailure(ctx, session, turnID, out)
		return afterSequence, true
	}

	completedReadFailures := 0
	for {
		query := store.EventQuery{
			TurnID: turnID, AfterSequence: afterSequence, Limit: promptDeliveryPageSize, Forward: true,
		}
		if throughSequence > 0 {
			query.TurnID = ""
		}
		persisted, err := recorder.Query(ctx, query)
		if errors.Is(err, store.ErrClosed) {
			return afterSequence, true
		}
		if err != nil {
			m.sessionLogger(session).WarnContext(ctx, "session: query prompt delivery catch-up failed", "error", err)
			if persistenceComplete {
				completedReadFailures++
				if completedReadFailures >= promptDeliveryReadAttempts {
					m.deliverPromptReadFailure(ctx, turnID, out)
					return afterSequence, true
				}
			}
		} else {
			completedReadFailures = 0
			for _, event := range persisted {
				terminal, _ := m.deliverPersistedPromptEvent(
					ctx, session, turnID, event, cards, persistenceDone, out,
				)
				afterSequence = max(afterSequence, event.Sequence)
				if terminal {
					return afterSequence, true
				}
			}
		}
		if err == nil && throughSequence > 0 && afterSequence >= throughSequence {
			return afterSequence, false
		}
		// A completed pump can still have several unread pages. Drain them before
		// diagnosing a missing terminal record or waiting for another live wake.
		if err == nil && len(persisted) == promptDeliveryPageSize {
			continue
		}
		if persistenceComplete && err == nil {
			m.deliverPromptProjectionFailure(ctx, session, turnID, out)
			return afterSequence, true
		}

		select {
		case <-ctx.Done():
			return afterSequence, true
		case <-persistenceDone:
			persistenceDone = nil
			persistenceComplete = true
		case <-ticker.C:
		}
	}
}

func waitForPromptPersistence(ctx context.Context, done <-chan struct{}) {
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (m *Manager) deliverPersistedPromptEvent(
	ctx context.Context,
	session *Session,
	turnID string,
	persisted store.SessionEvent,
	cards *transcript.SessionMessageCards,
	persistenceDone <-chan struct{},
	out chan<- acp.AgentEvent,
) (terminal bool, delivered bool) {
	if persisted.TurnID != turnID {
		return false, false
	}
	event, err := transcript.UnmarshalAgentEvent(persisted.Content)
	if err != nil {
		m.sessionLogger(session).ErrorContext(
			ctx,
			"session: decode durable prompt delivery event failed",
			"sequence", persisted.Sequence,
			"error", err,
		)
		m.deliverPromptProjectionFailure(ctx, session, turnID, out)
		return true, true
	}
	// The decoded event drops the persisted tool result; carry the card the
	// projection derives from the persisted form so the live prompt stream can
	// show it before the transcript reconciles (D-04, composer path).
	if card, ok := transcript.SubagentPartForStoredEvent(persisted); ok {
		if data, err := json.Marshal(card); err == nil {
			event = event.WithSubagentCard(data)
		}
	}
	if card, ok := cards.Card(persisted); ok {
		if data, err := json.Marshal(card); err == nil {
			event = event.WithSessionMessageCard(data)
		}
	}
	m.publishSubagentActivity(ctx, session, event)
	if event.Usage != nil {
		event.Usage.Sequence = persisted.Sequence
	}
	// The persisted user input anchors replay and history, but Prompt returns
	// only runtime output. A catch-up delivery must preserve that public contract.
	if !isPromptOutputEventType(event.Type) {
		return false, false
	}
	// Consumers may close their transport immediately on a terminal event.
	// Release prompt ownership before exposing that boundary to the next send.
	if isPromptTerminalEvent(event.Type) {
		waitForPromptPersistence(ctx, persistenceDone)
	}
	select {
	case out <- event:
		return isPromptTerminalEvent(event.Type), true
	case <-ctx.Done():
		return true, false
	}
}

func isPromptOutputEventType(eventType string) bool {
	switch eventType {
	case acp.EventTypeAgentMessage,
		acp.EventTypeThought,
		acp.EventTypeToolCall,
		acp.EventTypeToolResult,
		acp.EventTypePlan,
		acp.EventTypePermission,
		acp.EventTypeClarify,
		acp.EventTypeUsage,
		acp.EventTypeCompaction,
		acp.EventTypePromptDelivery,
		acp.EventTypeSystem,
		acp.EventTypeRuntimeProgress,
		acp.EventTypeRuntimeWarning,
		acp.EventTypeRuntimeRecoveryStarted,
		acp.EventTypeRuntimeRecoverySucceeded,
		acp.EventTypeRuntimeRecoveryExhausted,
		acp.EventTypeDone,
		acp.EventTypeError,
		acp.EventTypeAvailableCommands:
		return true
	default:
		return false
	}
}

func (m *Manager) deliverPromptProjectionFailure(
	ctx context.Context,
	session *Session,
	turnID string,
	out chan<- acp.AgentEvent,
) {
	event := acp.PromptStreamIncompleteEvent()
	event.TurnID = turnID
	if info := session.Info(); info.State != StateActive && info.Failure != nil &&
		info.Failure.Kind == store.FailurePrompt {
		event.Failure = store.CloneSessionFailure(info.Failure)
		event.Error = info.Failure.Summary
	}
	select {
	case out <- event:
	case <-ctx.Done():
	}
}

// deliverPromptReadFailure distinguishes unreadable history from a verified missing terminal.
func (m *Manager) deliverPromptReadFailure(ctx context.Context, turnID string, out chan<- acp.AgentEvent) {
	event := acp.PromptStreamIncompleteEvent()
	event.TurnID = turnID
	event.Error = "session: prompt delivery history could not be read"
	event.Failure.ReasonCode = store.FailureReasonPromptDeliveryReadFailed
	event.Failure.Summary = event.Error
	select {
	case out <- event:
	case <-ctx.Done():
	}
}
