package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
)

type promptRequest struct {
	turnID                 string
	target                 string
	message                string
	authoredMessage        string
	messageID              string
	idempotencyKey         string
	expectedTurnID         string
	eventID                string
	runtime                *RuntimeSelection
	turnSource             TurnSource
	meta                   acp.PromptMeta
	deliveryCtx            context.Context
	prepareDelivery        PromptDeliveryPreparer
	inputRecorded          bool
	releaseSlotBeforeHooks bool
	resumeStopped          bool
	commitDispatch         func(context.Context) error
	skillInvocations       []commandpkg.Invocation
	attachments            []AttachmentMeta
}

func (m *Manager) recordPromptInputEvent(
	ctx context.Context,
	session *Session,
	req *promptRequest,
) error {
	if req.inputRecorded {
		return nil
	}
	if strings.TrimSpace(req.authoredMessage) == "" && len(req.attachments) == 0 {
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
	event = event.WithPromptRuntime(promptRuntimeFromSelectionPointer(req.runtime))
	event = event.WithSkillInvocations(req.skillInvocations)
	event = event.WithAttachments(promptEventAttachments(req.attachments))
	if messageID := strings.TrimSpace(req.messageID); messageID != "" {
		event = event.WithMessageID(messageID)
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
		if err := m.recordPromptInputWithAuthoredText(
			ctx,
			session,
			event,
			req.authoredMessage,
			req.eventID,
		); err == nil {
			m.notifyAgentEvent(ctx, session, event)
			req.inputRecorded = true
			return nil
		} else if !errors.Is(err, store.ErrClosed) {
			return fmt.Errorf("session: persist prompt message for %q: %w", req.target, err)
		}
	}
	if err := m.recordInactivePromptInputEvent(ctx, req.target, event, req.authoredMessage, req.eventID); err != nil {
		return fmt.Errorf("session: persist prompt message for %q: %w", req.target, err)
	}
	req.inputRecorded = true
	return nil
}

func promptEventAttachments(attachments []AttachmentMeta) []acp.EventAttachment {
	if len(attachments) == 0 {
		return nil
	}
	result := make([]acp.EventAttachment, 0, len(attachments))
	for _, attachment := range attachments {
		result = append(result, acp.EventAttachment{
			ID:       attachment.ID,
			Name:     attachment.Name,
			MIMEType: attachment.MIMEType,
			Bytes:    attachment.Bytes,
			SHA256:   attachment.SHA256,
			Kind:     attachment.Kind,
			Width:    attachment.Width,
			Height:   attachment.Height,
		})
	}
	return result
}

func (m *Manager) recordInactivePromptInputEvent(
	ctx context.Context,
	sessionID string,
	event acp.AgentEvent,
	authoredText string,
	eventID string,
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
	persistedEventID, err := promptInputEventID(eventID)
	if err != nil {
		return err
	}
	persisted, err := m.appendDurableSessionEvent(ctx, sessionID, store.SessionEvent{
		ID:        persistedEventID,
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
	m.notifyAgentEventFromInfo(ctx, m.sessionInfoFromMeta(ctx, meta), event)
	return nil
}

func promptInputEventID(eventID string) (string, error) {
	if normalized := strings.TrimSpace(eventID); normalized != "" {
		return normalized, nil
	}
	generatedID, err := store.NewID("ev")
	if err != nil {
		return "", fmt.Errorf("session: generate prompt input event id: %w", err)
	}
	return generatedID, nil
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
