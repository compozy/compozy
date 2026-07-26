package transcript

import (
	"encoding/json"

	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

// UIMessageText returns the concatenated visible text parts for one UI message.
func UIMessageText(message UIMessage) string {
	parts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		if part.Type != uiPartText {
			continue
		}
		if strings.TrimSpace(part.Text) == "" {
			continue
		}
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "\n")
}

// JoinUIMessageText joins visible text parts across all messages with newlines.
func JoinUIMessageText(messages []UIMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if text := strings.TrimSpace(UIMessageText(message)); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func decodeStoredEvent(storedEvent store.SessionEvent) *decodedStoredEvent {
	decoded := &decodedStoredEvent{
		stored: storedEvent,
		parsed: parseEvent(storedEvent),
	}
	if event, err := UnmarshalAgentEvent(strings.TrimSpace(storedEvent.Content)); err == nil {
		decoded.agent = event
	} else {
		decoded.agent = fallbackAgentEvent(decoded.parsed, storedEvent)
	}

	if strings.TrimSpace(decoded.agent.Type) == "" {
		decoded.agent.Type = decoded.parsed.Type
	}
	if strings.TrimSpace(decoded.agent.SessionID) == "" {
		decoded.agent.SessionID = strings.TrimSpace(storedEvent.SessionID)
	}
	if strings.TrimSpace(decoded.agent.TurnID) == "" {
		decoded.agent.TurnID = firstNonEmpty(
			strings.TrimSpace(decoded.parsed.TurnID),
			strings.TrimSpace(storedEvent.TurnID),
		)
	}
	if decoded.agent.Timestamp.IsZero() && !storedEvent.Timestamp.IsZero() {
		decoded.agent.Timestamp = storedEvent.Timestamp.UTC()
	}
	if strings.TrimSpace(decoded.agent.Text) == "" {
		decoded.agent.Text = decoded.parsed.Text
	}
	if strings.TrimSpace(decoded.agent.ToolCallID) == "" {
		decoded.agent.ToolCallID = decoded.parsed.ToolCallID
	}
	if strings.TrimSpace(decoded.agent.Title) == "" {
		decoded.agent.Title = decoded.parsed.ToolName
	}
	if strings.TrimSpace(decoded.agent.Error) == "" && decoded.parsed.ToolResult != nil {
		decoded.agent.Error = firstNonEmpty(decoded.parsed.ToolResult.Error, decoded.parsed.Text)
	}
	decoded.parsed = redactTranscriptEvent(decoded.parsed)
	decoded.agent = RedactAgentEvent(decoded.agent)
	return decoded
}

func fallbackAgentEvent(parsed event, storedEvent store.SessionEvent) acp.AgentEvent {
	event := acp.AgentEvent{
		Type:       parsed.Type,
		SessionID:  strings.TrimSpace(storedEvent.SessionID),
		TurnID:     firstNonEmpty(strings.TrimSpace(parsed.TurnID), strings.TrimSpace(storedEvent.TurnID)),
		Timestamp:  storedEvent.Timestamp.UTC(),
		Text:       parsed.Text,
		Title:      parsed.ToolName,
		ToolCallID: parsed.ToolCallID,
	}

	content := strings.TrimSpace(storedEvent.Content)
	if content == "" {
		return event
	}
	if json.Valid([]byte(content)) {
		event.Raw = acp.CloneRawMessage(json.RawMessage(content))

		var payload map[string]any
		if err := json.Unmarshal([]byte(content), &payload); err == nil {
			event.RequestID = firstNonEmpty(
				nestedString(payload, "request_id"),
				nestedString(payload, "requestId"),
			)
			event.Action = nestedString(payload, "action")
			event.Resource = nestedString(payload, "resource")
			event.Decision = nestedString(payload, "decision")
			event.Error = firstNonEmpty(nestedString(payload, "error"), event.Error)
			event.StopReason = nestedString(payload, "stop_reason")
			event.PromptStopReason = acp.PromptStopReason(nestedString(payload, "prompt_stop_reason"))
		}
	}

	return event
}

func (d *decodedStoredEvent) dataPayload() json.RawMessage {
	if d == nil {
		return nil
	}
	payload := UIAgentEventPayloadFromEvent(d.agent)
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}
