package session

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

type inputClearEventPayload struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	TurnID      string `json:"turn_id"`
	EntryID     string `json:"queue_entry_id,omitempty"`
	Count       int    `json:"count"`
	ActorKind   string `json:"actor_kind"`
	ActorID     string `json:"actor_id"`
	Cause       string `json:"cause,omitempty"`
}

func (m *Manager) recordInputClearEvent(
	ctx context.Context, session *Session, eventType string, payload inputClearEventPayload, eventID string,
) error {
	info := session.Info()
	payload.WorkspaceID, payload.SessionID = info.WorkspaceID, info.ID
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	event := m.normalizeEvent(session, payload.TurnID, acp.AgentEvent{
		Type: eventType, SessionID: session.ID, Raw: raw, Timestamp: m.now(),
	})
	event = event.WithEventID(eventID)
	if err := m.recordEventWithWriter(ctx, session, event, "", recordIdempotentSessionEvent); err != nil {
		return err
	}
	m.notifyAgentEvent(ctx, session, event)
	return nil
}

func (m *Manager) recordInputClearFailure(
	ctx context.Context, session *Session, turnID string, actor PromptCaller, cause error,
) error {
	payload := inputClearEventPayload{TurnID: turnID, ActorKind: actor.Kind, ActorID: actor.ID, Cause: cause.Error()}
	if entryErr, ok := errors.AsType[*store.SessionInputClearError](cause); ok {
		payload.EntryID = entryErr.EntryID
	}
	id, err := store.NewID("queue-clear-failed")
	if err != nil {
		return errors.Join(cause, err)
	}
	return errors.Join(
		cause,
		m.recordInputClearEvent(context.WithoutCancel(ctx), session, events.SessionQueueClearFailed, payload, id),
	)
}
