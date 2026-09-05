package session

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
)

const stopEventScopeSession = "session"

type stopEventPayload struct {
	WorkspaceID string    `json:"workspace_id"`
	SessionID   string    `json:"session_id"`
	TurnID      string    `json:"turn_id"`
	Scope       string    `json:"scope"`
	Phase       StopPhase `json:"phase"`
	ElapsedMS   int64     `json:"elapsed_ms"`
	Cause       StopCause `json:"cause"`
	ActorID     string    `json:"actor_id,omitempty"`
	ActorKind   string    `json:"actor_kind,omitempty"`
	ReasonCode  string    `json:"reason_code,omitempty"`
}

func (m *Manager) recordSessionStopEscalation(
	ctx context.Context, session *Session, phase StopPhase, elapsed time.Duration, cause StopCause,
) error {
	session.mu.Lock()
	previouslyEscalated := session.stopEscalated
	session.stopEscalated = true
	session.mu.Unlock()
	var persistErr error
	if !previouslyEscalated || phase == StopPhaseForced {
		persistErr = m.persistSessionLifecycleState(ctx, session, false)
	}
	eventErr := m.recordStopEvent(ctx, session, events.SessionStopEscalated, stopEventPayload{
		Scope: stopEventScopeSession, Phase: phase, ElapsedMS: elapsed.Milliseconds(), Cause: cause,
	})
	return errors.Join(persistErr, eventErr)
}

func (m *Manager) recordSessionStopVerificationFailure(
	ctx context.Context, session *Session, outcome StopOutcome,
) error {
	return m.recordStopVerificationFailure(ctx, session, stopEventPayload{
		Scope:     stopEventScopeSession,
		Phase:     outcome.Phase,
		ElapsedMS: outcome.Elapsed.Milliseconds(),
		Cause:     outcome.Cause,
	})
}

func (m *Manager) recordStopVerificationFailure(ctx context.Context, session *Session, payload stopEventPayload) error {
	session.mu.Lock()
	if session.State == StateStopped {
		session.mu.Unlock()
		return nil
	}
	session.stopVerificationFailed = true
	session.UpdatedAt = m.now().UTC()
	session.mu.Unlock()
	persistErr := m.persistSessionLifecycleState(ctx, session, false)
	payload.ReasonCode = StopVerificationFailedCode
	eventErr := m.recordStopEvent(ctx, session, events.SessionStopVerificationFailed, payload)
	return errors.Join(persistErr, eventErr)
}

// sessionStopTurnID is the one turn identity an idle session's stop footprint
// shares: ladder escalation records and the terminal event land in the same
// history turn instead of each allocating their own.
func (m *Manager) sessionStopTurnID(session *Session) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.stopTurnID != "" {
		return session.stopTurnID, nil
	}
	turnID, err := m.newPromptTurnID()
	if err != nil {
		return "", err
	}
	session.stopTurnID = turnID
	return turnID, nil
}

func (m *Manager) recordStopEvent(
	ctx context.Context, session *Session, eventType string, payload stopEventPayload,
) error {
	info := session.Info()
	payload.WorkspaceID, payload.SessionID = info.WorkspaceID, info.ID
	if payload.TurnID == "" {
		payload.TurnID = session.CurrentTurnID()
	}
	if payload.TurnID == "" {
		turnID, err := m.sessionStopTurnID(session)
		if err != nil {
			return err
		}
		payload.TurnID = turnID
	}
	if actorID := actingSessionID(ctx); actorID != "" {
		payload.ActorID, payload.ActorKind = actorID, actingSessionActorKind
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := m.normalizeEvent(session, payload.TurnID, acp.AgentEvent{Type: eventType, Raw: raw, Timestamp: m.now()})
	if err := m.recordEvent(ctx, session, event); err != nil {
		return err
	}
	m.notifyAgentEvent(ctx, session, event)
	return nil
}
