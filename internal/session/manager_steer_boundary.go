package session

import (
	"context"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (m *Manager) boundarySteerDelivery(session *Session, entry *store.SessionInputQueueEntry) store.SteerDeliveryMode {
	if m.busyInput.SteerBoundaryTimeout == 0 || !store.IsHumanSessionInputOriginClass(entry.OriginClass) {
		return store.SteerDeliveryInterruptFallback
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.currentTurnID != entry.TargetTurnID {
		return store.SteerDeliveryAfterTurn
	}
	if session.openToolCallCountLocked() > 0 {
		return store.SteerDeliveryPendingBoundary
	}
	if !session.toolEventsObserved {
		return store.SteerDeliveryAfterTurn
	}
	return store.SteerDeliveryInterruptFallback
}

func (m *Manager) observeSteerToolBoundary(ctx context.Context, session *Session, event acp.AgentEvent) {
	if event.ToolCallID == "" || (event.Type != acp.EventTypeToolCall && event.Type != acp.EventTypeToolResult) {
		return
	}
	session.mu.Lock()
	if event.TurnID != session.currentTurnID {
		session.mu.Unlock()
		return
	}
	session.toolEventsObserved = true
	if event.Type == acp.EventTypeToolCall && event.ToolStatus() != string(acpsdk.ToolCallStatusCompleted) &&
		event.ToolStatus() != string(acpsdk.ToolCallStatusFailed) {
		if session.openToolCalls == nil {
			session.openToolCalls = make(map[string]bool)
		}
		if open, exists := session.openToolCalls[event.ToolCallID]; !exists || open {
			session.openToolCalls[event.ToolCallID] = true
		}
	} else {
		if session.openToolCalls == nil {
			session.openToolCalls = make(map[string]bool)
		}
		session.openToolCalls[event.ToolCallID] = false
	}
	boundary := session.openToolCallCountLocked() == 0 && session.hasBoundarySteer
	session.mu.Unlock()
	if !boundary {
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
			if err := m.activateBoundarySteer(ctx, session, entry.ID, event.TurnID); err != nil {
				m.sessionLogger(session).
					Warn("session: activate boundary steering failed", "entry_id", entry.ID, "error", err)
			}
		}
	}
}

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
		if err := m.activateBoundarySteer(ctx, session, entry.ID, entry.TargetTurnID); err != nil {
			m.sessionLogger(session).
				Error("session: activate boundary steering failed", "entry_id", entry.ID, "error", err)
		}
		return
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
		if err := m.activateBoundarySteer(ctx, session, entryID, turnID); err != nil {
			m.sessionLogger(session).
				Error("session: boundary steering timeout failed", "entry_id", entryID, "error", err)
		}
	})
}

func (m *Manager) activateBoundarySteer(ctx context.Context, session *Session, entryID, turnID string) error {
	entry, err := m.inputQueue.Get(ctx, session.ID, entryID)
	if err != nil {
		return err
	}
	if entry.Status != store.SessionInputQueueStatusQueued ||
		entry.SteerDelivery != store.SteerDeliveryPendingBoundary {
		return nil
	}
	if session.CurrentTurnID() != turnID {
		m.startNextQueuedInputPrompt(session.ID)
		return nil
	}
	resolved, changed, err := m.inputQueue.SettlePendingSteer(
		ctx,
		session.ID,
		entryID,
		store.SteerDeliveryInterruptFallback,
	)
	if err != nil || !changed {
		return err
	}
	m.emitSteerMarker(ctx, session, &resolved)
	return m.activateInterruptingInput(ctx, session, &resolved)
}

func (s *Session) openToolCallCountLocked() int {
	count := 0
	for _, open := range s.openToolCalls {
		if open {
			count++
		}
	}
	return count
}

func (m *Manager) supersedeBoundarySteers(
	ctx context.Context,
	session *Session,
	replacement *store.SessionInputQueueEntry,
) error {
	entries, err := m.inputQueue.List(ctx, session.ID)
	if err != nil {
		return err
	}
	for index := range entries {
		entry := &entries[index]
		if entry.ID == replacement.ID || entry.TargetTurnID != replacement.TargetTurnID ||
			entry.SteerDelivery != store.SteerDeliveryPendingBoundary ||
			!store.IsHumanSessionInputOriginClass(entry.OriginClass) {
			continue
		}
		if _, err := m.inputQueue.Cancel(ctx, session.ID, entry.ID); err != nil {
			return err
		}
		evidence := map[string]any{promptEvidenceQueueEntryIDKey: entry.ID, "replacement_entry_id": replacement.ID}
		addSteerAuthoredEvidence(evidence, entry)
		m.emitTranscriptMarker(ctx, session, entry.TargetTurnID, transcript.MarkerPromptSuperseded,
			"Undelivered steering replaced by an explicit interrupt.", evidence)
	}
	return nil
}
