package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

// watchSteerCompletion settles a pending injection once the provider reports its outcome. unresolved marks
// a pending steer whose resolution could not be saved at injection time; the watch saves it first, so the
// outcome still settles without sending the guidance again.
func (m *Manager) watchSteerCompletion(
	session *Session, entryID, targetTurnID string, completion <-chan error, unresolved bool,
) {
	m.startTrackedPromptTask(func() {
		deliveryErr, open := <-completion
		_, promoted := session.promotedSteers.LoadAndDelete(entryID)
		if !open {
			deliveryErr = errors.New("session: steer completed without a result")
		}
		delivery := store.SteerDeliveryInjected
		if deliveryErr != nil {
			delivery = store.SteerDeliveryInterruptFallback
		}
		ctx := m.fallbackLifecycleContext()
		defer func() {
			if service := m.ReplyWatches(); service != nil {
				service.OnSteerResolved(ctx, session.ID, "")
			}
		}()
		// Serialize the delivery decision with prepareStop's lifecycle transition.
		session.mu.Lock()
		if session.State != StateActive {
			session.mu.Unlock()
			if err := m.inputQueue.CancelPendingSteer(ctx, session.ID, entryID); err != nil {
				m.sessionLogger(session).Error("session: cancel stopped session steer failed", "error", err)
			}
			return
		}
		if unresolved && !m.savePendingSteer(ctx, session, entryID) {
			session.mu.Unlock()
			return
		}
		resolved, changed, err := m.inputQueue.SettlePendingSteer(ctx, session.ID, entryID, delivery)
		if err == nil && changed {
			session.steerDelivery = delivery
		}
		session.mu.Unlock()
		if err != nil {
			m.sessionLogger(session).
				Error("session: settle steer completion failed", "entry_id", entryID, "error", err)
			return
		}
		if !changed {
			return
		}
		if err := m.recordAcceptedSteerInput(ctx, session, &resolved); err != nil {
			m.sessionLogger(session).
				Error("session: persist confirmed guidance failed", "entry_id", entryID, "error", err)
		}
		m.emitSteerMarker(ctx, session, &resolved)
		if deliveryErr == nil {
			return
		}
		m.sessionLogger(session).Warn("session: pending steer failed; falling back",
			"entry_id", entryID, "error", deliveryErr)
		if promoted {
			// A long-turn promotion never holds the sender's queued-steer slot: return it to the queue.
			if _, err := m.inputQueue.RevertPromotedAgent(ctx, session.ID, entryID); err != nil {
				m.sessionLogger(session).
					Error("session: revert promoted queued input failed", "entry_id", entryID, "error", err)
			}
		}
		if steerFallbackWaitsForTurn(&resolved) || session.CurrentTurnID() != targetTurnID {
			m.startNextQueuedInputPrompt(session.ID)
			return
		}
		if err := m.activateInterruptingInput(ctx, session, &resolved); err != nil {
			m.sessionLogger(session).
				Error("session: activate pending steer fallback failed", "entry_id", entryID, "error", err)
		}
	})
}

// savePendingSteer saves the pending resolution an injection could not record, so the provider's outcome
// can settle it. A row still unsaved stays leased for boot recovery; it is never delivered again.
func (m *Manager) savePendingSteer(ctx context.Context, session *Session, entryID string) bool {
	_, err := m.resolveAcceptedSteer(ctx, session.ID, entryID, store.SteerDeliveryPendingInjection)
	if err != nil {
		m.sessionLogger(session).Error("session: save pending steer before settling failed",
			"entry_id", entryID, "error", err)
	}
	return err == nil
}
