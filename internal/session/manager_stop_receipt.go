package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (s *Session) retainVerifiedStopOutcome(outcome StopOutcome) StopOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopVerifiedOutcome.Verified {
		return s.stopVerifiedOutcome
	}
	if outcome.Verified {
		s.stopVerifiedOutcome = outcome
	}
	return outcome
}

// A process watcher can verify exit before the stop driver's call returns.
// Freeze that observation so the driver and recovery report the same result.
func (s *Session) stopReceiptOutcome(at time.Time) StopOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	outcome := s.stopVerifiedOutcome
	if !outcome.Verified {
		outcome.Verified = true
		if outcome.Phase == "" {
			outcome.Phase = StopPhaseCooperative
		}
		outcome.Escalated = s.stopEscalated
		if !s.stopStartedAt.IsZero() {
			outcome.Elapsed = max(0, at.Sub(s.stopStartedAt))
		}
	}
	outcome.FinalState, outcome.Cause = StateStopped, s.stopCause
	s.stopVerifiedOutcome = outcome
	return outcome
}

// persistActiveStopReceipt journals the exact terminal row before its append.
// Classification has already committed, so recovery preserves that metadata.
func (m *Manager) persistActiveStopReceipt(ctx context.Context, session *Session, event *store.SessionEvent) error {
	meta := session.Meta()
	outcome := session.stopReceiptOutcome(event.Timestamp)
	event.SessionID = meta.ID
	settlement := &recoveredStopSettlement{
		turnID: event.TurnID, startedAt: event.Timestamp.Add(-outcome.Elapsed),
		actorID: actingSessionID(ctx), outcome: outcome, detail: meta.StopDetail,
		receipt: recoveredStopReceipt{
			Version: 1, SessionID: meta.ID, WorkspaceID: meta.WorkspaceID,
			RuntimeGeneration: meta.RuntimeGeneration, CreatedAt: meta.CreatedAt, TerminalEvent: event,
		},
	}
	return m.writeRecoveredStopReceipt(meta.ID, settlement)
}

func (m *Manager) replayActiveStopEvent(ctx context.Context, id string, row *store.SessionEvent) error {
	var event acp.AgentEvent
	if err := json.Unmarshal([]byte(row.Content), &event); err != nil {
		return err
	}
	persisted, err := m.appendDurableSessionEvent(ctx, id, *row)
	if err != nil {
		return err
	}
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return err
	}
	if persisted.ID != row.ID {
		return errors.New("session: terminal replay changed event identity")
	}
	m.publishSessionEventByID(ctx, id, persisted)
	m.notifyAgentEventFromInfo(ctx, sessionInfoFromMeta(meta), event)
	return nil
}
