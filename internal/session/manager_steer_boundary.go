package session

import (
	"context"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// boundarySteerDelivery chooses a safe fallback from the current runtime observation.
func (m *Manager) boundarySteerDelivery(session *Session, entry *store.SessionInputQueueEntry) store.SteerDeliveryMode {
	session.mu.RLock()
	defer session.mu.RUnlock()
	return m.boundarySteerDeliveryLocked(session, entry)
}

// boundarySteerDeliveryLocked shares the decision with completion settlement under the session fence.
func (m *Manager) boundarySteerDeliveryLocked(
	session *Session,
	entry *store.SessionInputQueueEntry,
) store.SteerDeliveryMode {
	if m.busyInput.SteerBoundaryTimeout == 0 || !store.IsHumanSessionInputOriginClass(entry.OriginClass) {
		return store.SteerDeliveryInterruptFallback
	}
	if session.currentTurnID != entry.TargetTurnID {
		return store.SteerDeliveryAfterTurn
	}
	if !session.toolEventsObserved {
		return store.SteerDeliveryAfterTurn
	}
	return store.SteerDeliveryPendingBoundary
}

// trackSteerToolEvent tracks duplicate-safe tool lifetimes before event batching.
func (s *Session) trackSteerToolEvent(event acp.AgentEvent, turnID string) bool {
	if event.TurnID == "" {
		event.TurnID = turnID
	}
	if event.ToolCallID == "" || (event.Type != acp.EventTypeToolCall && event.Type != acp.EventTypeToolResult) {
		return false
	}
	s.mu.Lock()
	if event.TurnID != s.currentTurnID {
		s.mu.Unlock()
		return false
	}
	s.toolEventsObserved = true
	if event.Type == acp.EventTypeToolCall && event.ToolStatus() != string(acpsdk.ToolCallStatusCompleted) &&
		event.ToolStatus() != string(acpsdk.ToolCallStatusFailed) {
		if s.openToolCalls == nil {
			s.openToolCalls = make(map[string]bool)
		}
		if open, exists := s.openToolCalls[event.ToolCallID]; !exists || open {
			s.openToolCalls[event.ToolCallID] = true
		}
	} else {
		if s.openToolCalls == nil {
			s.openToolCalls = make(map[string]bool)
		}
		s.openToolCalls[event.ToolCallID] = false
	}
	boundary := s.openToolCallCountLocked() == 0 && s.hasBoundarySteer
	s.mu.Unlock()
	return boundary
}

// observeSteerToolBoundary activates waiting guidance after the boundary event is durable.
func (m *Manager) observeSteerToolBoundary(ctx context.Context, session *Session, event acp.AgentEvent) {
	if !session.trackSteerToolEvent(event, event.TurnID) {
		return
	}
	entries, err := m.inputQueue.List(ctx, session.ID)
	if err != nil {
		m.sessionLogger(session).Warn("session: list boundary steering failed", "error", err)
		return
	}
	for index := range entries {
		entry := &entries[index]
		if entry.SteerDelivery == store.SteerDeliveryPendingBoundary && entry.TargetTurnID == event.TurnID {
			if err := m.activateBoundarySteer(ctx, session, entry.ID, event.TurnID, false); err != nil {
				m.sessionLogger(session).
					Warn("session: activate boundary steering failed", "entry_id", entry.ID, "error", err)
			}
		}
	}
}

// watchSteerBoundary races tool completion against a lifecycle-owned timeout.
func (m *Manager) watchSteerBoundary(session *Session, entry *store.SessionInputQueueEntry) {
	session.mu.Lock()
	if session.currentTurnID != entry.TargetTurnID {
		session.mu.Unlock()
		m.startNextQueuedInputPrompt(session.ID)
		return
	}
	session.hasBoundarySteer = true
	boundary := session.currentTurnID == entry.TargetTurnID && session.openToolCallCountLocked() == 0
	done := session.currentPromptDone
	session.mu.Unlock()
	ctx := m.fallbackLifecycleContext()
	if boundary {
		if err := m.activateBoundarySteer(ctx, session, entry.ID, entry.TargetTurnID, false); err != nil {
			m.sessionLogger(session).
				Error("session: activate boundary steering failed", "entry_id", entry.ID, "error", err)
		}
	}
	entryID, turnID := entry.ID, entry.TargetTurnID
	m.startTrackedPromptTask(func() {
		timer := time.NewTimer(m.busyInput.SteerBoundaryTimeout)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-timer.C:
		}
		if err := m.activateBoundarySteer(ctx, session, entryID, turnID, true); err != nil {
			m.sessionLogger(session).
				Error("session: boundary steering timeout failed", "entry_id", entryID, "error", err)
		}
	})
}

// activateBoundarySteer retries only uncommitted timeout settlement; cancellation is claimed once.
func (m *Manager) activateBoundarySteer(
	ctx context.Context,
	session *Session,
	entryID, turnID string,
	expired bool,
) error {
	for delay := queuedInputSentRetryDelay; ; delay = min(2*delay, queuedInputSentRetryMaxDelay) {
		resolved, run, owned, err := m.claimBoundarySteer(ctx, session, entryID, turnID, expired)
		if err == nil {
			if resolved != nil {
				m.emitSteerMarker(ctx, session, resolved)
			}
			if owned {
				m.startTurnStop(ctx, run, CauseUserRequested)
			}
			return nil
		}
		if !expired {
			return err
		}
		m.sessionLogger(session).
			Warn("session: boundary steer settlement failed; retrying", "entry_id", entryID, "error", err)
		timer := time.NewTimer(delay)
		session.mu.RLock()
		done := session.currentPromptDone
		session.mu.RUnlock()
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-done:
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

// claimBoundarySteer fences observed tool starts with settlement and the shared cancellation claim.
func (m *Manager) claimBoundarySteer(
	ctx context.Context,
	session *Session,
	entryID, turnID string,
	expired bool,
) (*store.SessionInputQueueEntry, *turnStopRun, bool, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.State != StateActive || session.currentTurnID != turnID ||
		(!expired && session.openToolCallCountLocked() > 0) {
		return nil, nil, false, nil
	}
	entry, err := m.inputQueue.Get(ctx, session.ID, entryID)
	if err != nil {
		return nil, nil, false, err
	}
	if entry.Status != store.SessionInputQueueStatusQueued ||
		entry.SteerDelivery != store.SteerDeliveryPendingBoundary {
		return nil, nil, false, nil
	}
	resolved, changed, err := m.inputQueue.SettlePendingSteer(
		ctx,
		session.ID,
		entryID,
		store.SteerDeliveryInterruptFallback,
	)
	if err != nil || !changed {
		return nil, nil, false, err
	}
	session.steerDelivery = resolved.SteerDelivery
	run, owned, err := session.claimTurnStopLocked(turnID)
	return &resolved, run, owned, err
}

// openToolCallCountLocked derives the count without maintaining a second mutable counter.
func (s *Session) openToolCallCountLocked() int {
	count := 0
	for _, open := range s.openToolCalls {
		if open {
			count++
		}
	}
	return count
}
