package session

import (
	"context"
	"encoding/json"

	"github.com/compozy/compozy/internal/events"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

const (
	sessionDaemonActorID   = "daemon"
	sessionSystemActorKind = "system"
)

func (m *Manager) recordSupervisionEvent(
	ctx context.Context, target *Session, kind string, state *SupervisionState,
) error {
	event, err := m.prepareSupervisionEvent(target, kind, state)
	if err != nil {
		return err
	}
	if err := m.recordEventWithWriter(ctx, target, event, "", recordIdempotentSessionEvent); err != nil {
		return err
	}
	m.notifyAgentEvent(ctx, target, event)
	return nil
}

func (m *Manager) prepareSupervisionEvent(
	target *Session, kind string, state *SupervisionState,
) (acp.AgentEvent, error) {
	target.mu.RLock()
	cached := target.supervisionWarningEvent
	if kind == events.SessionSupervisionStopped {
		cached = target.supervisionStoppedEvent
	}
	sameWarning := kind == events.SessionSupervisionWarning && state.QuietWarning != nil &&
		cached != nil && cached.Timestamp.Equal(state.QuietWarning.WarnedAt)
	if cached != nil && (kind == events.SessionSupervisionStopped || sameWarning) {
		event := *cached
		target.mu.RUnlock()
		return event, nil
	}
	target.mu.RUnlock()
	turnID := target.CurrentTurnID()
	if turnID == "" {
		var err error
		turnID, err = m.sessionStopTurnID(target)
		if err != nil {
			return acp.AgentEvent{}, err
		}
	}
	info := target.Info()
	raw, err := json.Marshal(struct {
		WorkspaceID string            `json:"workspace_id"`
		SessionID   string            `json:"session_id"`
		TurnID      string            `json:"turn_id"`
		ActorID     string            `json:"actor_id"`
		ActorKind   string            `json:"actor_kind"`
		Cause       string            `json:"cause,omitempty"`
		Supervision *SupervisionState `json:"supervision"`
	}{info.WorkspaceID, info.ID, turnID, sessionDaemonActorID, sessionSystemActorKind, stopDetailInactivity, state})
	if err != nil {
		return acp.AgentEvent{}, err
	}
	eventID, err := m.newPromptTurnID()
	if err != nil {
		return acp.AgentEvent{}, err
	}
	event := m.normalizeEvent(target, turnID, acp.AgentEvent{
		Type: kind, Raw: raw, Timestamp: m.now(),
		EventCorrelation: store.EventCorrelation{ActorKind: sessionSystemActorKind, ActorID: sessionDaemonActorID},
	}).WithEventID(kind + ":" + eventID)
	if kind == events.SessionSupervisionStopped {
		event = event.WithEventID(kind + ":" + target.ID + ":" + turnID)
	} else if state.QuietWarning != nil {
		episode := state.QuietWarning.WarnedAt.Format("20060102T150405.999999999Z07:00")
		event = event.WithEventID(kind + ":" + target.ID + ":" + episode)
	}

	if kind == events.SessionSupervisionWarning && state.QuietWarning != nil {
		event.Timestamp = state.QuietWarning.WarnedAt
	}
	target.mu.Lock()
	if kind == events.SessionSupervisionWarning {
		target.supervisionWarningEvent = &event
	}
	if kind == events.SessionSupervisionStopped {
		target.supervisionStoppedEvent = &event
	}
	target.mu.Unlock()
	return event, nil
}
