package transcript

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/mention"
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
	goal := event.GoalPromptMeta()
	origin := event.PromptOrigin()
	invocations := inputUISkillInvocations(event.SkillInvocations())
	mentions := inputUIMentions(event.Mentions())
	attachments := event.Attachments()
	if event.Synthetic == nil && turnID == "" && messageID == "" && goal == nil && origin == nil &&
		len(invocations) == 0 && len(mentions) == 0 &&
		len(attachments) == 0 {
		return nil
	}
	timestamp := ""
	if !event.Timestamp.IsZero() {
		timestamp = event.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	encoded, err := json.Marshal(struct {
		Synthetic        *inputUISyntheticMeta    `json:"synthetic,omitempty"`
		TurnID           string                   `json:"turn_id,omitempty"`
		Timestamp        string                   `json:"timestamp,omitempty"`
		MessageID        string                   `json:"message_id,omitempty"`
		Origin           *acp.PromptOriginMeta    `json:"origin,omitempty"`
		Goal             *acp.GoalPromptMeta      `json:"goal,omitempty"`
		SkillInvocations []inputUISkillInvocation `json:"skill_invocations,omitempty"`
		Mentions         []inputUIMention         `json:"mentions,omitempty"`
		Attachments      []acp.EventAttachment    `json:"attachments,omitempty"`
	}{
		Synthetic: inputUISynthetic(event.Synthetic),
		TurnID:    turnID, Timestamp: timestamp, MessageID: messageID, Goal: goal, Origin: origin,
		SkillInvocations: invocations, Mentions: mentions, Attachments: attachments,
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

// inputUIMention is the transcript wire shape of a verified mention. Target is
// relative to the session working directory or a session id; never absolute.
type inputUIMention struct {
	Kind   string `json:"kind"`
	Token  string `json:"token"`
	Target string `json:"target"`
	Label  string `json:"label"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
}

func inputUIMentions(mentions []mention.Mention) []inputUIMention {
	result := make([]inputUIMention, 0, len(mentions))
	for _, item := range mentions {
		result = append(result, inputUIMention{
			Kind: string(item.Kind), Token: item.Token, Target: item.Target,
			Label: item.Label, Start: item.Start, End: item.End,
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

type inputUISyntheticMeta struct {
	Kind             string `json:"kind"`
	WakeEventID      string `json:"wake_event_id,omitempty"`
	ChildSessionID   string `json:"child_session_id,omitempty"`
	ChildWorkspaceID string `json:"child_workspace_id,omitempty"`
	ChildAgentName   string `json:"child_agent_name,omitempty"`
	Reason           string `json:"reason,omitempty"`
	ReplyTruncated   bool   `json:"reply_truncated,omitzero"`
	Hop              int    `json:"hop,omitzero"`
	Summary          string `json:"summary,omitempty"`
}

func inputUISynthetic(meta *acp.PromptSyntheticMeta) *inputUISyntheticMeta {
	normalized := clonePromptSyntheticMeta(meta)
	if normalized == nil {
		return nil
	}
	result := &inputUISyntheticMeta{
		Kind: normalized.Kind, WakeEventID: normalized.WakeEventID,
		ChildSessionID: normalized.ChildSessionID, ChildWorkspaceID: normalized.ChildWorkspaceID,
		ChildAgentName: normalized.ChildAgentName, Reason: normalized.Reason,
		ReplyTruncated: normalized.ReplyTruncated, Hop: normalized.Hop,
	}
	if normalized.Kind == acp.PromptSyntheticKindSessionReply {
		result.Summary = normalized.Summary
	}
	return result
}
