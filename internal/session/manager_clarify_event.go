package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/transcript"
)

const EventTypeClarify = acp.EventTypeClarify

// clarifyPendingWire mirrors toolspkg.ClarifyPending with a nullable Deadline
// so unbounded waits render null instead of the year-1 epoch.
type clarifyPendingWire struct {
	toolspkg.ClarifyPending
	Deadline *time.Time `json:"deadline"`
}

// clarifyEventWire mirrors toolspkg.ClarifyEvent with the nullable-deadline request.
type clarifyEventWire struct {
	toolspkg.ClarifyEvent
	Request clarifyPendingWire `json:"request"`
}

func clarifyEventWirePayload(event toolspkg.ClarifyEvent) clarifyEventWire {
	wire := clarifyEventWire{ClarifyEvent: event}
	wire.Request = clarifyPendingWire{ClarifyPending: event.Request}
	if !event.Request.Deadline.IsZero() {
		deadline := event.Request.Deadline
		wire.Request.Deadline = &deadline
	}
	return wire
}

// PublishClarifyEvent durably records one typed clarification transition before SSE publication.
func (m *Manager) PublishClarifyEvent(ctx context.Context, event toolspkg.ClarifyEvent) error {
	_, err := m.publishClarifyEvent(ctx, event, toolspkg.Scope{})
	return err
}

// PublishClarifyAnswer returns the queue wake to run after the broker releases the request.
func (m *Manager) PublishClarifyAnswer(
	ctx context.Context,
	event toolspkg.ClarifyEvent,
	askingScope toolspkg.Scope,
) (func(), error) {
	return m.publishClarifyEvent(ctx, event, askingScope)
}

func (m *Manager) publishClarifyEvent(
	ctx context.Context,
	event toolspkg.ClarifyEvent,
	askingScope toolspkg.Scope,
) (func(), error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: clarification event context is required")
	}
	if event.Status == toolspkg.ClarifyStatusResolved {
		event.ResolvedBy = resolvedByFromContext(ctx, pendingInteractionActorOperator)
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(event.Request.SessionID)
	active, ok := m.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, sessionID)
	}
	info := active.Info()
	if info.WorkspaceID != strings.TrimSpace(event.Request.WorkspaceID) ||
		info.AgentName != strings.TrimSpace(event.Request.AgentName) {
		return nil, fmt.Errorf("session: clarification ownership mismatch for %q", sessionID)
	}
	attentionCommitted, queued, err := m.applyClarifyAttentionEvent(ctx, active, event, askingScope)
	if err != nil {
		return nil, fmt.Errorf("session: persist canonical clarification attention: %w", err)
	}
	var wake func()
	if queued {
		wake = func() { m.startNextQueuedInputPrompt(sessionID) }
	}
	payload, err := json.Marshal(clarifyEventWirePayload(event))
	if err != nil {
		return wake, m.handleClarifyTranscriptFailure(ctx, active, event, attentionCommitted, err)
	}
	turnID := "clarify:" + event.Request.RequestID
	content, err := transcript.MarshalAgentEvent(acp.AgentEvent{
		Type:      EventTypeClarify,
		SessionID: sessionID,
		TurnID:    turnID,
		Timestamp: event.At.UTC(),
		Raw:       payload,
	}.WithRequestID(event.Request.RequestID).WithResolvedBy(event.ResolvedBy))
	if err != nil {
		return wake, m.handleClarifyTranscriptFailure(ctx, active, event, attentionCommitted, err)
	}
	persisted, _, err := m.appendDurableSessionEvent(ctx, sessionID, store.SessionEvent{
		ID:        event.Request.RequestID + "-" + string(event.Status),
		SessionID: sessionID,
		TurnID:    turnID,
		Type:      EventTypeClarify,
		AgentName: info.AgentName,
		Content:   content,
		Timestamp: event.At.UTC(),
	})
	if err != nil {
		return wake, m.handleClarifyTranscriptFailure(ctx, active, event, attentionCommitted, err)
	}
	m.publishSessionEvent(ctx, active, persisted)
	return wake, nil
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
