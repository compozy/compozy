package transcript

import (
	"encoding/json/v2"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolmeta"
)

type UISessionMessagePayload struct {
	ToolCallID        string `json:"tool_call_id"`
	TargetSessionID   string `json:"target_session_id"`
	TargetWorkspaceID string `json:"target_workspace_id,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Delivery          string `json:"delivery,omitempty"`
	ReplyWatchID      string `json:"reply_watch_id,omitempty"`
	State             string `json:"state"`
}

func (b *uiMessageBuilder) appendSessionMessagePart(decoded *decodedStoredEvent) {
	payload, ok := sessionMessagePartPayload(decoded)
	if !ok {
		return
	}
	for _, part := range b.parts {
		if part.Type != "data-compozy-session-message" || part.ID != payload.ToolCallID {
			continue
		}
		var prior UISessionMessagePayload
		if json.Unmarshal(part.Data, &prior) == nil {
			payload = payload.WithFallback(prior)
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	b.appendDataPart("data-compozy-session-message", payload.ToolCallID, data)
}

func SessionMessagePartForStoredEvent(stored store.SessionEvent) (UISessionMessagePayload, bool) {
	return sessionMessagePartPayload(decodeStoredEvent(stored))
}

func sessionMessagePartPayload(decoded *decodedStoredEvent) (UISessionMessagePayload, bool) {
	event := decoded.agent
	providerTool := toolmeta.NormalizeHostedToolName(event.ProviderToolName())
	parsedTool := toolmeta.NormalizeHostedToolName(decoded.parsed.ToolName)
	if event.ToolCallID == "" ||
		(providerTool != "compozy__session_prompt" && parsedTool != "compozy__session_prompt") {
		return UISessionMessagePayload{}, false
	}
	payload := UISessionMessagePayload{ToolCallID: event.ToolCallID, State: "running"}
	var input struct {
		SessionID   string `json:"session_id"`
		WorkspaceID string `json:"workspace_id"`
		MessageID   string `json:"message_id"`
		Mode        string `json:"mode"`
	}
	if json.Unmarshal(event.ToolInput(), &input) == nil {
		payload.TargetSessionID, payload.MessageID, payload.Mode = input.SessionID, input.MessageID, input.Mode
		payload.TargetWorkspaceID = input.WorkspaceID
	}
	switch decoded.parsed.Type {
	case acp.EventTypeToolCall:
	case acp.EventTypeToolResult:
		payload.State = "done"
		if decoded.parsed.ToolError {
			payload.State = "error"
		}
		if result := decoded.parsed.ToolResult; result != nil {
			var envelope struct {
				Prompt sessionMessageOutput `json:"prompt"`
			}
			raw := result.RawOutput
			if len(raw) == 0 {
				raw = []byte(result.Content)
			}
			var output sessionMessageOutput
			if json.Unmarshal(raw, &envelope) == nil && envelope.Prompt.MessageID != "" {
				output = envelope.Prompt
			} else if err := json.Unmarshal(raw, &output); err != nil {
				return payload, true
			}
			if output.TargetWorkspaceID != "" {
				payload.TargetWorkspaceID = output.TargetWorkspaceID
			}
			if output.MessageID != "" {
				payload.MessageID = output.MessageID
			}
			if output.Mode != "" {
				payload.Mode = output.Mode
			}
			payload.Delivery = output.Delivery
			if output.ReplyWatch != nil {
				payload.ReplyWatchID = output.ReplyWatch.ID
			}
		}
	default:
		return UISessionMessagePayload{}, false
	}
	return payload, true
}

type sessionMessageOutput struct {
	TargetWorkspaceID string `json:"target_workspace_id"`
	MessageID         string `json:"message_id"`
	Mode              string `json:"mode"`
	Delivery          string `json:"delivery"`
	ReplyWatch        *struct {
		ID string `json:"id"`
	} `json:"reply_watch"`
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
