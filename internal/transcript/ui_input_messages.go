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
	hasText := strings.TrimSpace(text) != ""
	if !hasText && len(attachments) == 0 {
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
	if hasText {
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
	synthetic := inputUISyntheticMetadataFromEvent(event.Synthetic)
	invocations := inputUISkillInvocations(event.SkillInvocations())
	attachments := event.Attachments()
	if turnID == "" && messageID == "" && goal == nil && synthetic == nil &&
		len(invocations) == 0 && len(attachments) == 0 {
		return nil
	}
	encoded, err := json.Marshal(struct {
		TurnID           string                    `json:"turn_id,omitempty"`
		MessageID        string                    `json:"message_id,omitempty"`
		Goal             *acp.GoalPromptMeta       `json:"goal,omitempty"`
		Synthetic        *inputUISyntheticMetadata `json:"synthetic,omitempty"`
		SkillInvocations []inputUISkillInvocation  `json:"skill_invocations,omitempty"`
		Attachments      []acp.EventAttachment     `json:"attachments,omitempty"`
	}{
		TurnID: turnID, MessageID: messageID, Goal: goal, Synthetic: synthetic,
		SkillInvocations: invocations, Attachments: attachments,
	})
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}

// inputUISyntheticMetadata is the public, storage-opaque portion of one
// daemon-authored call or mailbox turn. The durable ACP record carries more
// runtime bookkeeping, including payload references and policy hashes; those
// fields do not belong in the browser transcript.
type inputUISyntheticMetadata struct {
	CallID         string `json:"call_id,omitempty"`
	CallState      string `json:"call_state,omitempty"`
	ChildSessionID string `json:"child_session_id,omitempty"`
	ChildAgentName string `json:"child_agent_name,omitempty"`
	ResultBytes    int    `json:"result_bytes,omitempty"`
	ContractDigest string `json:"contract_digest,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	DeliveryKind   string `json:"delivery_kind,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Summary        string `json:"summary,omitempty"`
	WakeEventID    string `json:"wake_event_id,omitempty"`
}

func inputUISyntheticMetadataFromEvent(meta *acp.PromptSyntheticMeta) *inputUISyntheticMetadata {
	if meta == nil {
		return nil
	}
	normalized := meta.Normalize()
	if normalized.CallID == "" && normalized.MessageID == "" {
		return nil
	}
	return &inputUISyntheticMetadata{
		CallID: normalized.CallID, CallState: normalized.CallState,
		ChildSessionID: normalized.ChildSessionID, ChildAgentName: normalized.ChildAgentName,
		ResultBytes: normalized.ResultBytes, ContractDigest: normalized.ContractDigest,
		MessageID: normalized.MessageID, DeliveryKind: normalized.DeliveryKind,
		Reason: normalized.Reason, Summary: normalized.Summary, WakeEventID: normalized.WakeEventID,
	}
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
