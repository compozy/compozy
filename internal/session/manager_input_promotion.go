package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// promoteLongTurnQueuedInputs delivers queued messages from other sessions into a target whose turn
// has outlived session.messaging.queue_promote_after, as injected steering. A long Goal or
// orchestration turn may never reach "after the turn", so without this the message would wait for
// as long as the turn runs. Promotion keeps each entry's identity, so its reply watch still binds to
// the turn that consumes it. It never runs while operator input waits and never interrupts the turn:
// an injection the provider refuses degrades back to after-turn delivery.
func (m *Manager) promoteLongTurnQueuedInputs(ctx context.Context, session *Session) {
	if m == nil || m.inputQueue == nil || session == nil {
		return
	}
	after := m.messagingConfig().QueuePromoteAfter
	if after <= 0 {
		return
	}
	turnID, startedAt, ok := session.activeTurnStart()
	if !ok || m.now().Sub(startedAt) < after {
		return
	}
	if _, ok := m.injectedSteerer(session.processHandle()); !ok {
		return
	}
	if !session.inputPromotionActive.CompareAndSwap(false, true) {
		return
	}
	defer session.inputPromotionActive.Store(false)
	entries, err := m.inputQueue.List(ctx, session.ID)
	if err != nil {
		m.sessionLogger(session).Warn("session: list queued input for promotion failed", "error", err)
		return
	}
	if operatorInputWaits(entries) {
		// Operator input is ahead; promoting past it would let another session cut the line.
		return
	}
	// Injection runs on the live binding: an entry that asked for another runtime waits for its own turn.
	live := session.runtimeBindingSnapshot().selection
	liveRuntime := storeRuntimeSelection(&live)
	effective := effectiveQueuedInputRuntime(session)
	for index := range entries {
		entry := &entries[index]
		if !promotableQueuedInput(entry) || !sameSessionInputRuntime(effective(entry.Runtime), liveRuntime) {
			continue
		}
		if declined, ok := session.promotionDeclined.Load(entry.ID); ok && declined == turnID {
			continue
		}
		delivery, promoted := m.promoteQueuedInput(ctx, session, entry, turnID)
		if promoted && delivery != store.SteerDeliveryInjected {
			// A pending or refused injection settles first; the next pass continues.
			return
		}
	}
}

func promotableQueuedInput(entry *store.SessionInputQueueEntry) bool {
	return entry.OwnerKind == "" && entry.OriginClass == store.SessionInputOriginClassAgent &&
		entry.Mode == store.SessionInputQueueModeQueue && entry.Status == store.SessionInputQueueStatusQueued &&
		len(entry.SkillInvocations) == 0
}

func (m *Manager) promoteQueuedInput(
	ctx context.Context,
	session *Session,
	entry *store.SessionInputQueueEntry,
	turnID string,
) (store.SteerDeliveryMode, bool) {
	entryID := entry.ID
	staged, promoted, err := m.inputQueue.PromoteQueuedAgentToSteer(ctx, session.ID, entryID, turnID)
	if err != nil {
		m.sessionLogger(session).Warn("session: promote queued input failed", "entry_id", entryID, "error", err)
		return "", false
	}
	if !promoted {
		return "", false
	}
	// The same input.pre_submit review an ordinary dispatch applies, only once the promotion is staged: a
	// refusal returns the message to the queue for that dispatch to settle, and a rewrite is what the
	// provider receives.
	message, err := m.dispatchInputPreSubmit(
		ctx,
		session,
		turnID,
		TurnSourceUser,
		entry.Text,
		attachmentMetaFromStore(entry.Attachments),
	)
	if err != nil {
		session.promotionDeclined.Store(entryID, turnID)
		_, revertErr := m.revertPromotedQueuedInput(ctx, session, entryID)
		m.sessionLogger(session).Warn("session: input hook declined queued input promotion",
			"entry_id", entryID, "error", err, "revert_error", revertErr)
		return "", false
	}
	providerText := ""
	if message != entry.Text {
		providerText = message
	}
	session.promotedSteers.Store(entryID, struct{}{})
	err = m.activateSteeringInputText(ctx, session, &staged, providerText)
	// A pending injection keeps its completion watch, even when its resolution could not be saved, and
	// that watch returns a rejected promotion to the queue.
	watched := staged.SteerDelivery == store.SteerDeliveryPendingInjection &&
		(err == nil || errors.Is(err, errSteerDeliveredUnresolved))
	if !watched {
		session.promotedSteers.Delete(entryID)
	}
	if err == nil && staged.SteerDelivery != store.SteerDeliveryInterruptFallback {
		return staged.SteerDelivery, true
	}
	if errors.Is(err, errSteerDeliveredUnresolved) {
		// The provider already holds the message: returning it to the queue would deliver it twice.
		m.sessionLogger(session).Error("session: promoted queued input delivered but not resolved",
			"entry_id", entryID, "error", err, "steer_delivery", staged.SteerDelivery)
		return staged.SteerDelivery, true
	}
	// Never drop, cancel, or interrupt with the message: a failed or fallen-back injection returns it to
	// plain after-turn delivery, releasing the sender's queued-steer slot.
	reverted, revertErr := m.revertPromotedQueuedInput(ctx, session, entryID)
	m.sessionLogger(session).Warn(
		"session: promoted queued input not injected; deferring to after the turn",
		"entry_id", entryID, "error", err, "steer_delivery", staged.SteerDelivery,
		"reverted", reverted, "revert_error", revertErr,
	)
	return "", true
}

// revertPromotedQueuedInput returns a staged promotion to after-turn delivery. The staged row was hidden
// from ordinary dispatch, so a turn that ended meanwhile skipped it; dispatch restarts once it is back.
func (m *Manager) revertPromotedQueuedInput(ctx context.Context, session *Session, entryID string) (bool, error) {
	reverted, err := m.inputQueue.RevertPromotedAgent(context.WithoutCancel(ctx), session.ID, entryID)
	if err == nil && reverted {
		m.startNextQueuedInputPrompt(session.ID)
	}
	return reverted, err
}

// schedulePromoteLongTurnQueuedInputs runs one promotion pass outside the caller's lock order.
func (m *Manager) schedulePromoteLongTurnQueuedInputs(session *Session) {
	if m == nil || session == nil || m.messagingConfig().QueuePromoteAfter <= 0 {
		return
	}
	m.startTrackedPromptTask(func() {
		m.promoteLongTurnQueuedInputs(m.fallbackLifecycleContext(), session)
	})
}

// activeTurnStart reports the live turn and when its runtime activity started.
func (s *Session) activeTurnStart() (string, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentTurnID == "" || s.Liveness == nil || s.Liveness.Activity == nil {
		return "", time.Time{}, false
	}
	activity := s.Liveness.Activity
	if activity.TurnID != s.currentTurnID || activity.TurnStartedAt == nil || activity.TurnStartedAt.IsZero() {
		return "", time.Time{}, false
	}
	return s.currentTurnID, activity.TurnStartedAt.UTC(), true
}
