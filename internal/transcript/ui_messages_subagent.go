package transcript

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subagentid"
)

// UISubagentPayload identifies the roster entry and supplies initial display hints.
type UISubagentPayload struct {
	SubagentID      string `json:"subagent_id"`
	ToolCallID      string `json:"tool_call_id"`
	TurnID          string `json:"turn_id"`
	Origin          string `json:"origin,omitempty"`
	Title           string `json:"title,omitempty"`
	RuntimeProvider string `json:"runtime_provider,omitempty"`
	RuntimeAgent    string `json:"runtime_agent,omitempty"`
	RuntimeModel    string `json:"runtime_model,omitempty"`
}

func (b *uiMessageBuilder) appendSubagentPart(decoded *decodedStoredEvent) {
	event := decoded.agent
	payload := UISubagentPayload{ToolCallID: event.ToolCallID}
	switch {
	case decoded.parsed.Type == acp.EventTypeToolCall &&
		(event.ProviderToolName() == "Agent" || event.ProviderToolName() == "Task"):
		parentID := firstNonEmpty(decoded.stored.SessionID, event.SessionID)
		if parentID == "" || event.ToolCallID == "" {
			return
		}
		payload.SubagentID = subagentid.Derive(parentID, event.ToolCallID)
		payload.Origin = store.SubagentOriginProviderNative
		payload.Title = event.Title
		payload.RuntimeProvider = "claude"
		var input struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(event.ToolInput(), &input) == nil {
			payload.RuntimeModel = input.Model
		}
	case decoded.parsed.Type == acp.EventTypeToolResult && !decoded.parsed.ToolError &&
		(decoded.parsed.ToolName == "compozy__subagent_delegate" || event.ProviderToolName() == "compozy__subagent_delegate"):
		result := decoded.parsed.ToolResult
		if result == nil {
			return
		}
		if json.Unmarshal(result.RawOutput, &payload) != nil || payload.SubagentID == "" {
			if json.Unmarshal([]byte(result.Content), &payload) != nil || payload.SubagentID == "" {
				return
			}
		}
		payload.ToolCallID = event.ToolCallID
		payload.Origin = store.SubagentOriginDelegated
	default:
		return
	}
	payload.TurnID = event.TurnID
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	b.appendDataPart("data-compozy-subagent", payload.SubagentID, data)
}
