package transcript

import (
	"encoding/json/v2"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolmeta"
)

const (
	sessionMessagePartType = "data-compozy-session-message"
	sessionPromptToolID    = "compozy__session_prompt"
)

type UISessionMessagePayload struct {
	ToolCallID        string `json:"tool_call_id"`
	TargetSessionID   string `json:"target_session_id"`
	TargetWorkspaceID string `json:"target_workspace_id,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Delivery          string `json:"delivery,omitempty"`
	SteerDelivery     string `json:"steer_delivery,omitempty"`
	ReplyWatchID      string `json:"reply_watch_id,omitempty"`
	State             string `json:"state"`
}

func (b *uiMessageBuilder) appendSessionMessagePart(decoded *decodedStoredEvent) {
	prior, known := b.sessionMessagePart(decoded.agent.ToolCallID)
	payload, ok := sessionMessagePartPayload(decoded, prior, known)
	if !ok {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	b.appendDataPart(sessionMessagePartType, payload.ToolCallID, data)
	// A send made inside a provider-native subagent stays attributed to it, like the call's tool part.
	if parent := decoded.agent.ParentToolCallID(); parent != "" {
		for index := range b.parts {
			if b.parts[index].Type == sessionMessagePartType && b.parts[index].ID == payload.ToolCallID {
				b.parts[index].ParentToolCallID = parent
			}
		}
	}
}

// sessionMessagePart returns the card this entry already projected for a call.
func (b *uiMessageBuilder) sessionMessagePart(toolCallID string) (UISessionMessagePayload, bool) {
	if toolCallID == "" {
		return UISessionMessagePayload{}, false
	}
	for _, part := range b.parts {
		if part.Type != sessionMessagePartType || part.ID != toolCallID {
			continue
		}
		var prior UISessionMessagePayload
		if json.Unmarshal(part.Data, &prior) == nil {
			return prior, true
		}
	}
	return UISessionMessagePayload{}, false
}

// SessionMessagePartForStoredEvent projects one event on its own. It knows no earlier event
// of the call, so a result that names no tool (Codex) needs SessionMessageCards instead.
func SessionMessagePartForStoredEvent(stored store.SessionEvent) (UISessionMessagePayload, bool) {
	return sessionMessagePartPayload(decodeStoredEvent(stored), UISessionMessagePayload{}, false)
}

// SessionMessageCards projects the card of each event in one ordered stream the way the
// transcript entry does: a call's later events resolve against the card its earlier events
// named. The zero value is ready to use; it is not safe for concurrent use.
type SessionMessageCards struct {
	cards map[string]UISessionMessagePayload
}

// Card returns the call's card after this event, or false when the event is not part of a
// session message the card can name yet.
func (c *SessionMessageCards) Card(stored store.SessionEvent) (UISessionMessagePayload, bool) {
	decoded := decodeStoredEvent(stored)
	prior, known := c.cards[decoded.agent.ToolCallID]
	payload, ok := sessionMessagePartPayload(decoded, prior, known)
	if !ok {
		return UISessionMessagePayload{}, false
	}
	if c.cards == nil {
		c.cards = make(map[string]UISessionMessagePayload)
	}
	c.cards[payload.ToolCallID] = payload
	return payload, true
}

// sessionMessagePartPayload folds one event of a `compozy__session_prompt` call into its card.
// known reports that an earlier event of the call already projected one (prior): Codex reports
// the result without the tool's name, so the call's identity comes from that card.
func sessionMessagePartPayload(
	decoded *decodedStoredEvent,
	prior UISessionMessagePayload,
	known bool,
) (UISessionMessagePayload, bool) {
	event := decoded.agent
	if event.ToolCallID == "" || (!known && !isSessionPromptEvent(decoded)) {
		return UISessionMessagePayload{}, false
	}
	payload := UISessionMessagePayload{ToolCallID: event.ToolCallID, State: "running"}
	input := sessionPromptArguments(event.ToolInput())
	payload.TargetSessionID, payload.TargetWorkspaceID = input.SessionID, input.WorkspaceID
	payload.MessageID, payload.Mode = input.MessageID, input.Mode
	switch decoded.parsed.Type {
	case acp.EventTypeToolCall:
	case acp.EventTypeToolResult:
		payload.State = "done"
		if decoded.parsed.ToolError {
			payload.State = "error"
		}
		if output, ok := sessionPromptResultOutput(decoded.parsed.ToolResult); ok {
			payload = payload.withOutput(output)
		}
	default:
		return UISessionMessagePayload{}, false
	}
	if known {
		payload = payload.WithFallback(prior)
	}
	// The card names its target (S3). Claude streams the arguments, so its first events carry
	// none yet: the call stays a tool row until one does, never a card without a target.
	if payload.TargetSessionID == "" {
		return UISessionMessagePayload{}, false
	}
	return payload, true
}

// IsSessionPromptCall reports a stored tool call of `compozy__session_prompt`, under any provider's
// name for the hosted tool.
func IsSessionPromptCall(stored store.SessionEvent) bool {
	decoded := decodeStoredEvent(stored)
	return decoded.parsed.Type == acp.EventTypeToolCall && isSessionPromptEvent(decoded)
}

func isSessionPromptEvent(decoded *decodedStoredEvent) bool {
	return toolmeta.NormalizeHostedToolName(decoded.agent.ProviderToolName()) == sessionPromptToolID ||
		toolmeta.NormalizeHostedToolName(decoded.parsed.ToolName) == sessionPromptToolID
}

func (p UISessionMessagePayload) withOutput(output sessionMessageOutput) UISessionMessagePayload {
	if output.TargetWorkspaceID != "" {
		p.TargetWorkspaceID = output.TargetWorkspaceID
	}
	if output.MessageID != "" {
		p.MessageID = output.MessageID
	}
	if output.Mode != "" {
		p.Mode = output.Mode
	}
	p.Delivery = output.Delivery
	p.SteerDelivery = output.SteerDelivery
	if output.ReplyWatch != nil {
		p.ReplyWatchID = output.ReplyWatch.ID
	}
	return p
}

func (p UISessionMessagePayload) WithFallback(prior UISessionMessagePayload) UISessionMessagePayload {
	if p.TargetWorkspaceID == "" {
		p.TargetWorkspaceID = prior.TargetWorkspaceID
	}
	if p.TargetSessionID == "" {
		p.TargetSessionID = prior.TargetSessionID
	}
	if p.MessageID == "" {
		p.MessageID = prior.MessageID
	}
	if p.Mode == "" {
		p.Mode = prior.Mode
	}
	return p
}
