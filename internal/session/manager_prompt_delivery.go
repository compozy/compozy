package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const promptDeliveryCatchUpInterval = 25 * time.Millisecond

type durablePromptDelivery struct {
	events          <-chan acp.AgentEvent
	persistenceDone chan struct{}
	cancel          context.CancelFunc
}

func (m *Manager) openDurablePromptDelivery(
	ctx context.Context,
	session *Session,
	turnID string,
) (durablePromptDelivery, error) {
	deliveryCtx, cancel := context.WithCancel(ctx)
	persisted, unsubscribe, err := m.subscribePersistedSessionEvents(deliveryCtx, session.ID, 0)
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
			persisted,
			unsubscribe,
			persistenceDone,
			out,
		)
	})
	return durablePromptDelivery{events: out, persistenceDone: persistenceDone, cancel: cancel}, nil
}

func (m *Manager) runDurablePromptDelivery(
	ctx context.Context,
	session *Session,
	turnID string,
	live <-chan store.SessionEvent,
	unsubscribe func(),
	persistenceDone <-chan struct{},
	out chan<- acp.AgentEvent,
) {
	defer close(out)
	defer unsubscribe()

	afterSequence := int64(0)
	persistenceComplete := false
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
			terminal, delivered := m.deliverPersistedPromptEvent(ctx, session, turnID, persisted, out)
			if persisted.Sequence > afterSequence {
				afterSequence = persisted.Sequence
			}
			if delivered && terminal {
				waitForPromptPersistence(ctx, persistenceDone)
				return
			}
		}
	}

	m.catchUpDurablePromptDelivery(
		ctx,
		session,
		turnID,
		afterSequence,
		persistenceDone,
		persistenceComplete,
		out,
	)
}

func (m *Manager) catchUpDurablePromptDelivery(
	ctx context.Context,
	session *Session,
	turnID string,
	afterSequence int64,
	persistenceDone <-chan struct{},
	persistenceComplete bool,
	out chan<- acp.AgentEvent,
) {
	ticker := time.NewTicker(promptDeliveryCatchUpInterval)
	defer ticker.Stop()
	recorder := session.recorderHandle()
	if recorder == nil {
		m.deliverPromptProjectionFailure(ctx, turnID, out)
		return
	}

	for {
		persisted, err := recorder.Query(ctx, store.EventQuery{
			TurnID:        turnID,
			AfterSequence: afterSequence,
		})
		if err != nil {
			m.sessionLogger(session).WarnContext(ctx, "session: query prompt delivery catch-up failed", "error", err)
		} else {
			for _, event := range persisted {
				terminal, delivered := m.deliverPersistedPromptEvent(ctx, session, turnID, event, out)
				if event.Sequence > afterSequence {
					afterSequence = event.Sequence
				}
				if delivered && terminal {
					waitForPromptPersistence(ctx, persistenceDone)
					return
				}
			}
		}
		if persistenceComplete {
			m.deliverPromptProjectionFailure(ctx, turnID, out)
			return
		}

		select {
		case <-ctx.Done():
			return
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
		m.deliverPromptProjectionFailure(ctx, turnID, out)
		return true, true
	}
	// The persisted user input anchors replay and history, but Prompt returns
	// only runtime output. A catch-up delivery must preserve that public contract.
	if !isPromptOutputEventType(event.Type) {
		return false, false
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
	turnID string,
	out chan<- acp.AgentEvent,
) {
	event := acp.PromptStreamIncompleteEvent()
	event.TurnID = turnID
	select {
	case out <- event:
	case <-ctx.Done():
	}
}
