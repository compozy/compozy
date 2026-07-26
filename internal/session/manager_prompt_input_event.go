package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

type promptRequest struct {
	turnID          string
	target          string
	message         string
	authoredMessage string
	clientMessageID string
	turnSource      TurnSource
	meta            acp.PromptMeta
	deliveryCtx     context.Context
	prepareDelivery PromptDeliveryPreparer
	inputRecorded   bool
}

func (m *Manager) recordPromptInputEvent(
	ctx context.Context,
	session *Session,
	req *promptRequest,
) error {
	if req.inputRecorded {
		return nil
	}
	if strings.TrimSpace(req.authoredMessage) == "" {
		return errors.New("session: authored prompt message is required")
	}
	if session == nil {
		if active, ok := m.Get(req.target); ok {
			session = active
		}
	}

	event := acp.AgentEvent{
		Type:      acp.EventTypeUserMessage,
		TurnID:    req.turnID,
		Timestamp: m.now(),
		Text:      req.message,
	}
	if clientMessageID := strings.TrimSpace(req.clientMessageID); clientMessageID != "" {
		event = event.WithClientMessageID(clientMessageID)
	}
	if req.turnSource == TurnSourceSynthetic {
		event.Type = acp.EventTypeSyntheticReentry
		event.Synthetic = clonePromptSyntheticMeta(req.meta.Synthetic)
		if event.Synthetic != nil {
			event.Synthetic.Goal = nil
		}
	}
	event = m.normalizeEvent(session, req.turnID, event)
	if session != nil {
		if err := m.recordEventWithAuthoredText(ctx, session, event, req.authoredMessage); err == nil {
			m.notifyAgentEvent(ctx, session, event)
			req.inputRecorded = true
			return nil
		} else if !errors.Is(err, store.ErrClosed) {
			return fmt.Errorf("session: persist prompt message for %q: %w", req.target, err)
		}
	}
	if err := m.recordInactivePromptInputEvent(ctx, req.target, event, req.authoredMessage); err != nil {
		return fmt.Errorf("session: persist prompt message for %q: %w", req.target, err)
	}
	req.inputRecorded = true
	return nil
}

func (m *Manager) recordInactivePromptInputEvent(
	ctx context.Context,
	sessionID string,
	event acp.AgentEvent,
	authoredText string,
) error {
	meta, err := m.readMetaWithContext(ctx, sessionID)
	if err != nil {
		return err
	}
	event.SessionID = sessionID
	payload, err := marshalAgentEventPreservingAuthoredText(event, authoredText)
	if err != nil {
		return err
	}
	persisted, err := m.appendDurableSessionEvent(ctx, sessionID, store.SessionEvent{
		ID:        store.NewID("ev"),
		SessionID: sessionID,
		TurnID:    event.TurnID,
		Type:      event.Type,
		AgentName: meta.AgentName,
		Content:   payload,
		Timestamp: event.Timestamp,
	})
	if err != nil {
		return err
	}
	m.publishSessionEventByID(ctx, sessionID, persisted)
	if m.notifier != nil {
		m.notifier.OnAgentEvent(ctx, sessionID, event)
	}
	return nil
}

func clonePromptSyntheticMeta(meta *acp.PromptSyntheticMeta) *acp.PromptSyntheticMeta {
	if meta == nil {
		return nil
	}
	cloned := meta.Normalize()
	if cloned.IsZero() {
		return nil
	}
	return &cloned
}
