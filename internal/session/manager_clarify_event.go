package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/transcript"
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
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("session: marshal clarification event: %w", err)
	}
	turnID := "clarify:" + event.Request.RequestID
	content, err := transcript.MarshalAgentEvent(acp.AgentEvent{
		Type:      EventTypeClarify,
		SessionID: sessionID,
		TurnID:    turnID,
		RequestID: event.Request.RequestID,
		Timestamp: event.At.UTC(),
		Raw:       payload,
	})
	if err != nil {
		return fmt.Errorf("session: encode clarification event: %w", err)
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
		return fmt.Errorf("session: persist clarification event: %w", err)
	}
	m.publishSessionEvent(ctx, active, persisted)
	return nil
}
