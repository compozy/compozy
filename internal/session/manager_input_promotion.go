package session

import (
	"context"
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
	for index := range entries {
		entry := &entries[index]
		if entry.OwnerKind == "" && store.IsHumanSessionInputOriginClass(entry.OriginClass) {
			// Operator input is ahead; promoting past it would let another session cut the line.
			return
		}
		if !promotableQueuedInput(entry) {
			continue
		}
		delivery, promoted := m.promoteQueuedInput(ctx, session, entry.ID, turnID)
		if promoted && delivery != store.SteerDeliveryInjected {
			// A pending or refused injection settles first; the next pass continues.
			return
		}
	}
}

func promotableQueuedInput(entry *store.SessionInputQueueEntry) bool {
	return entry.OwnerKind == "" && entry.OriginClass == store.SessionInputOriginClassAgent &&
		entry.Mode == store.SessionInputQueueModeQueue && entry.Status == store.SessionInputQueueStatusQueued &&
		len(entry.Attachments) == 0 && len(entry.SkillInvocations) == 0
}

func (m *Manager) promoteQueuedInput(
	ctx context.Context,
	session *Session,
	entryID string,
	turnID string,
) (store.SteerDeliveryMode, bool) {
	staged, promoted, err := m.inputQueue.PromoteQueuedAgentToSteer(ctx, session.ID, entryID, turnID)
	if err != nil {
		m.sessionLogger(session).Warn("session: promote queued input failed", "entry_id", entryID, "error", err)
		return "", false
	}
	if !promoted {
		return "", false
	}
	session.promotedSteers.Store(entryID, struct{}{})
	err = m.activateSteeringInput(ctx, session, &staged)
	if err != nil || staged.SteerDelivery != store.SteerDeliveryPendingInjection {
		session.promotedSteers.Delete(entryID)
	}
	if err == nil && staged.SteerDelivery != store.SteerDeliveryInterruptFallback {
		return staged.SteerDelivery, true
	}
	// Never drop, cancel, or interrupt with the message: a failed or fallen-back injection returns it to
	// plain after-turn delivery, releasing the sender's queued-steer slot.
	reverted, revertErr := m.inputQueue.RevertPromotedAgent(context.WithoutCancel(ctx), session.ID, entryID)
	m.sessionLogger(session).Warn(
		"session: promoted queued input not injected; deferring to after the turn",
		"entry_id", entryID, "error", err, "steer_delivery", staged.SteerDelivery,
		"reverted", reverted, "revert_error", revertErr,
	)
	return "", true
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
