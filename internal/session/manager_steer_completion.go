package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// watchSteerCompletion settles a pending injection once the provider reports its outcome. unresolved marks
// a pending steer whose resolution could not be saved at injection time; the watch saves it first, so the
// outcome still settles without sending the guidance again.
func (m *Manager) watchSteerCompletion(
	session *Session, entryID string, completion <-chan error, unresolved bool,
) {
	m.startTrackedPromptTask(func() {
		deliveryErr, open := <-completion
		_, promoted := session.promotedSteers.LoadAndDelete(entryID)
		if !open {
			deliveryErr = errors.New("session: steer completed without a result")
		}
		delivery := store.SteerDeliveryInjected
		if deliveryErr != nil {
			session.forgetCreatorSteerInput(entryID)
			delivery = store.SteerDeliveryInterruptFallback
		}
		ctx := m.fallbackLifecycleContext()
		defer func() {
			if service := m.ReplyWatches(); service != nil {
				service.OnSteerResolved(ctx, session.ID, "")
			}
		}()
		resolved, changed, err := m.settleSteerCompletion(ctx, session, entryID, delivery, unresolved)
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
		if err := m.activateSteerFallback(ctx, session, &resolved); err != nil {
			m.sessionLogger(session).
				Error("session: activate pending steer fallback failed", "entry_id", entryID, "error", err)
		}
	})
}

// settleSteerCompletion retains the lease and provider outcome while storage recovers. Each attempt
// serializes with Stop, but logging and backoff happen without the session lock.
func (m *Manager) settleSteerCompletion(
	ctx context.Context, session *Session, entryID string, delivery store.SteerDeliveryMode, unresolved bool,
) (store.SessionInputQueueEntry, bool, error) {
	for delay := queuedInputSentRetryDelay; ; delay = min(2*delay, queuedInputSentRetryMaxDelay) {
		// Serialize the delivery decision with prepareStop's lifecycle transition.
		session.mu.Lock()
		if session.State != StateActive {
			session.mu.Unlock()
			return store.SessionInputQueueEntry{}, false, m.inputQueue.CancelPendingSteer(ctx, session.ID, entryID)
		}
		var (
			resolved store.SessionInputQueueEntry
			changed  bool
			err      error
		)
		if unresolved {
			resolved, err = m.resolveAcceptedSteer(ctx, session.ID, entryID, store.SteerDeliveryPendingInjection)
			unresolved = err != nil
			if err == nil && resolved.SupersededBy != "" {
				session.mu.Unlock()
				return resolved, false, nil
			}
		}
		settledDelivery := delivery
		if err == nil && delivery == store.SteerDeliveryInterruptFallback {
			var pending store.SessionInputQueueEntry
			pending, err = m.inputQueue.Get(ctx, session.ID, entryID)
			if err == nil {
				settledDelivery = m.boundarySteerDeliveryLocked(session, &pending)
			}
		}
		if err == nil {
			resolved, changed, err = m.inputQueue.SettlePendingSteer(ctx, session.ID, entryID, settledDelivery)
		}
		if err == nil && changed {
			session.steerDelivery = settledDelivery
		}
		session.mu.Unlock()
		if err == nil {
			return resolved, changed, nil
		}
		m.sessionLogger(session).Warn("session: settle steer completion failed; retrying",
			"entry_id", entryID, "error", err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return store.SessionInputQueueEntry{}, false, ctx.Err()
		case <-timer.C:
		}
	}
}
