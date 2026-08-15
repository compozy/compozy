package transcript

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	commandpkg "github.com/compozy/compozy/internal/command"
)

func inputUIMessage(decoded *decodedStoredEvent, role string) *UIMessage {
	text := decoded.parsed.Text
	attachments := decoded.agent.Attachments()
	if strings.TrimSpace(text) == "" && len(attachments) == 0 {
		return nil
	}
	parts := make([]UIMessagePart, 0, len(attachments)+1)
	for _, attachment := range attachments {
		parts = append(parts, UIMessagePart{
			Type:      UIMessagePartTypeFile,
			MediaType: attachment.MIMEType,
			URL:       attachmentspkg.AttachmentURIPrefix + attachment.ID,
			Filename:  attachment.Name,
		})
	}
	if strings.TrimSpace(text) != "" {
		parts = append(parts, UIMessagePart{
			Type:  uiPartText,
			Text:  text,
			State: uiPartStateDone,
		})
	}
	return &UIMessage{
		ID:       inputMessageID(decoded, role),
		Role:     role,
		Metadata: inputUIMessageMetadata(decoded.agent),
		Parts:    parts,
	}
}

func inputUIMessageMetadata(event acp.AgentEvent) json.RawMessage {
	turnID := strings.TrimSpace(event.TurnID)
	messageID := event.MessageIDValue()
	goal := acp.CloneGoalPromptMeta(event.Goal)
	invocations := inputUISkillInvocations(event.SkillInvocations())
	attachments := event.Attachments()
	if turnID == "" && messageID == "" && goal == nil && len(invocations) == 0 && len(attachments) == 0 {
		return nil
	}
	encoded, err := json.Marshal(struct {
		TurnID           string                   `json:"turn_id,omitempty"`
		MessageID        string                   `json:"message_id,omitempty"`
		Goal             *acp.GoalPromptMeta      `json:"goal,omitempty"`
		SkillInvocations []inputUISkillInvocation `json:"skill_invocations,omitempty"`
		Attachments      []acp.EventAttachment    `json:"attachments,omitempty"`
	}{
		TurnID: turnID, MessageID: messageID, Goal: goal,
		SkillInvocations: invocations, Attachments: attachments,
	})
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}

type inputUISkillInvocation struct {
	CommandID string `json:"command_id"`
	Token     string `json:"token"`
	Label     string `json:"label"`
	Source    string `json:"source"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
}

func inputUISkillInvocations(invocations []commandpkg.Invocation) []inputUISkillInvocation {
	result := make([]inputUISkillInvocation, 0, len(invocations))
	for _, invocation := range invocations {
		source := invocation.Ref.Source.Kind
		if invocation.Ref.Source.ID != "" {
			source += ":" + invocation.Ref.Source.ID
		}
		result = append(result, inputUISkillInvocation{
			CommandID: invocation.Ref.CommandID,
			Token:     invocation.Token,
			Label:     invocation.Ref.Name,
			Source:    source,
			Start:     invocation.Start,
			End:       invocation.End,
		})
	}
	return result
}

// Markers use the assistant data-compozy-event wire contract.
func runtimeMarkerUIMessage(decoded *decodedStoredEvent) UIMessage {
	return UIMessage{
		ID: fallbackMessageID(
			strings.TrimSpace(decoded.stored.ID),
			strings.TrimSpace(decoded.parsed.ID),
			"runtime-marker",
		),
		Role: UIRoleAssistant,
		Parts: []UIMessagePart{{
			Type: uiPartDataEvent,
			Data: decoded.dataPayload(),
		}},
	}
}
