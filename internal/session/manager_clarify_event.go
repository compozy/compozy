package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/transcript"
)

const EventTypeClarify = acp.EventTypeClarify

// PublishClarifyEvent durably records one typed clarification transition before SSE publication.
func (m *Manager) PublishClarifyEvent(ctx context.Context, event toolspkg.ClarifyEvent) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: clarification event context is required")
	}
	if event.Status == toolspkg.ClarifyStatusResolved {
		event.ResolvedBy = resolvedByFromContext(ctx, pendingInteractionActorOperator)
	}
	if err := event.Validate(); err != nil {
		return err
	}
	sessionID := strings.TrimSpace(event.Request.SessionID)
	active, ok := m.Get(sessionID)
	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, sessionID)
	}
	info := active.Info()
	if info.WorkspaceID != strings.TrimSpace(event.Request.WorkspaceID) ||
		info.AgentName != strings.TrimSpace(event.Request.AgentName) {
		return fmt.Errorf("session: clarification ownership mismatch for %q", sessionID)
	}
	attentionCommitted, err := m.applyClarifyAttentionEvent(ctx, active, event)
	if err != nil {
		return fmt.Errorf("session: persist canonical clarification attention: %w", err)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return m.handleClarifyTranscriptFailure(ctx, active, event, attentionCommitted, err)
	}
	turnID := "clarify:" + event.Request.RequestID
	content, err := transcript.MarshalAgentEvent(acp.AgentEvent{
		Type:       EventTypeClarify,
		SessionID:  sessionID,
		TurnID:     turnID,
		Timestamp:  event.At.UTC(),
		ResolvedBy: event.ResolvedBy,
		Raw:        payload,
	}.WithRequestID(event.Request.RequestID))
	if err != nil {
		return m.handleClarifyTranscriptFailure(ctx, active, event, attentionCommitted, err)
	}
	persisted, err := m.appendDurableSessionEvent(ctx, sessionID, store.SessionEvent{
		ID:        event.Request.RequestID + "-" + string(event.Status),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      EventTypeClarify,
		AgentName: info.AgentName,
		Content:   content,
		Timestamp: event.At.UTC(),
	})
	if err != nil {
		return m.handleClarifyTranscriptFailure(ctx, active, event, attentionCommitted, err)
	}
	m.publishSessionEvent(ctx, active, persisted)
	return nil
}

func (m *Manager) handleClarifyTranscriptFailure(
	ctx context.Context,
	target *Session,
	event toolspkg.ClarifyEvent,
	attentionCommitted bool,
	err error,
) error {
	if !attentionCommitted {
		return err
	}
	m.sessionLogger(target).WarnContext(
		ctx,
		"session: append clarification transcript event failed after canonical commit",
		"request_id", event.Request.RequestID,
		"status", event.Status,
		"error", err,
	)
	return nil
}
