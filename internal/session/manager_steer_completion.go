package session

import (
	"errors"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) watchSteerCompletion(session *Session, entryID, targetTurnID string, completion <-chan error) {
	m.startTrackedPromptTask(func() {
		deliveryErr, open := <-completion
		if !open {
			deliveryErr = errors.New("session: steer completed without a result")
		}
		delivery := store.SteerDeliveryInjected
		if deliveryErr != nil {
			delivery = store.SteerDeliveryInterruptFallback
		}
		ctx := m.fallbackLifecycleContext()
		// Serialize the delivery decision with prepareStop's lifecycle transition.
		session.mu.Lock()
		if session.State != StateActive {
			session.mu.Unlock()
			if err := m.inputQueue.CancelPendingSteer(ctx, session.ID, entryID); err != nil {
				m.sessionLogger(session).Error("session: cancel stopped session steer failed", "error", err)
			}
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
		m.emitSteerMarker(ctx, session, &resolved)
		if deliveryErr == nil {
			return
		}
		m.sessionLogger(session).Warn("session: pending steer failed; falling back",
			"entry_id", entryID, "error", deliveryErr)
		if session.CurrentTurnID() != targetTurnID {
			m.startNextQueuedInputPrompt(session.ID)
			return
		}
		if err := m.activateInterruptingInput(ctx, session, &resolved); err != nil {
			m.sessionLogger(session).
				Error("session: activate pending steer fallback failed", "entry_id", entryID, "error", err)
		}
	})
}
