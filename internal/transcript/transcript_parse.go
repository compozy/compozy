package transcript

import (
	"encoding/json"

	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

func parseEvent(sessionEvent store.SessionEvent) event {
	parsed := event{
		ID:        strings.TrimSpace(sessionEvent.ID),
		TurnID:    strings.TrimSpace(sessionEvent.TurnID),
		Type:      strings.TrimSpace(sessionEvent.Type),
		Timestamp: sessionEvent.Timestamp.UTC(),
	}

	content := strings.TrimSpace(sessionEvent.Content)
	if content == "" {
		return redactTranscriptEvent(parsed)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		if parsed.Type == acp.EventTypeUserMessage || parsed.Type == acp.EventTypeAgentMessage ||
			parsed.Type == acp.EventTypeThought {
			parsed.Text = content
			return redactTranscriptEvent(parsed)
		}
		return redactTranscriptEvent(parsed)
	}

	if schema := nestedString(payload, "schema"); schema == CanonicalSchema {
		return redactTranscriptEvent(parseCanonicalEvent(parsed, payload))
	}
	if _, ok := payload["sessionUpdate"]; ok {
		return redactTranscriptEvent(parseLegacyEvent(parsed, payload))
	}
	return redactTranscriptEvent(parseLooseEvent(parsed, payload))
}

func parseCanonicalEvent(parsed event, payload map[string]any) event {
	parsed.Type = firstNonEmpty(nestedString(payload, "type"), parsed.Type)
	parsed.Text = firstNonEmpty(nestedString(payload, "authored_text"), nestedString(payload, "text"))
	parsed.StopReason = nestedString(payload, "stop_reason")
	parsed.Error = nestedString(payload, "error")
	parsed.Failure = sessionFailureFromValue(payload["failure"])
	parsed.Runtime = runtimeActivityFromValue(payload["runtime"])
	if parsed.Type == events.TranscriptMarkerCreated || parsed.Type == events.TranscriptMarkerRedacted {
		parsed.Marker = parseMarkerFromPayload(payload)
	}
	parsed.ToolCallID = firstNonEmpty(nestedString(payload, "tool_call_id"), nestedString(payload, "toolCallId"))
	parsed.ToolName = firstNonEmpty(nestedString(payload, "tool_name"), nestedString(payload, "title"))
	parsed.ToolInput = acp.CloneRawMessage(rawMessageFromValue(payload["tool_input"]))
	if toolResult := decodeToolResult(rawMessageFromValue(payload["tool_result"])); toolResult != nil {
		parsed.ToolResult = toolResult
	}
	parsed.ToolError = nestedBool(payload, "tool_error") || strings.TrimSpace(nestedString(payload, "error")) != ""
	if parsed.ToolResult != nil && strings.TrimSpace(parsed.ToolResult.Error) != "" {
		parsed.ToolError = true
	}
	return parsed
}

func parseLegacyEvent(parsed event, payload map[string]any) event {
	updateType := nestedString(payload, "sessionUpdate")
	status := strings.ToLower(strings.TrimSpace(nestedString(payload, "status")))
	parsed.Text = extractLegacyContentText(payload["content"])
	parsed.ToolCallID = firstNonEmpty(nestedString(payload, "toolCallId"), nestedString(payload, "tool_call_id"))
	parsed.ToolName = legacyToolName(payload)
	parsed.ToolInput = acp.CloneRawMessage(rawMessageFromValue(payload["rawInput"]))

	switch updateType {
	case "user_message_chunk":
		parsed.Type = acp.EventTypeUserMessage
	case "agent_message_chunk":
		parsed.Type = acp.EventTypeAgentMessage
	case "agent_thought_chunk":
		parsed.Type = acp.EventTypeThought
	case "tool_call":
		parsed.Type = acp.EventTypeToolCall
	case "tool_call_update":
		if parsed.Type != acp.EventTypeToolResult {
			if status == "completed" || status == "failed" {
				parsed.Type = acp.EventTypeToolResult
			} else {
				parsed.Type = acp.EventTypeToolCall
			}
		}
	}

	if parsed.Type == acp.EventTypeToolResult {
		parsed.ToolResult = buildToolResult(
			parsed.ToolName,
			strings.EqualFold(status, "failed"),
			extractLegacyContentText(payload["content"]),
			payload["rawOutput"],
		)
		parsed.ToolError = strings.EqualFold(status, "failed")
	}

	return parsed
}

func parseLooseEvent(parsed event, payload map[string]any) event {
	parsed.Type = firstNonEmpty(nestedString(payload, "type"), parsed.Type)
	parsed.Text = nestedString(payload, "text")
	parsed.StopReason = nestedString(payload, "stop_reason")
	parsed.Error = nestedString(payload, "error")
	parsed.Failure = sessionFailureFromValue(payload["failure"])
	parsed.Runtime = runtimeActivityFromValue(payload["runtime"])
	if parsed.Type == events.TranscriptMarkerCreated || parsed.Type == events.TranscriptMarkerRedacted {
		parsed.Marker = parseMarkerFromPayload(payload)
	}
	parsed.ToolCallID = firstNonEmpty(nestedString(payload, "tool_call_id"), nestedString(payload, "toolCallId"))
	parsed.ToolName = firstNonEmpty(
		nestedString(payload, "tool_name"),
		nestedString(payload, "title"),
		legacyToolName(payload),
	)
	parsed.ToolInput = acp.CloneRawMessage(firstNonEmptyRaw(
		rawMessageFromValue(payload["tool_input"]),
		rawMessageFromValue(payload["rawInput"]),
		rawMessageFromValue(payload["raw"]),
	))

	if toolResult := decodeToolResult(firstNonEmptyRaw(
		rawMessageFromValue(payload["tool_result"]),
		rawMessageFromValue(payload["toolResult"]),
	)); toolResult != nil {
		parsed.ToolResult = toolResult
	} else if parsed.Type == acp.EventTypeToolResult {
		parsed.ToolResult = buildToolResult(
			parsed.ToolName,
			strings.TrimSpace(nestedString(payload, "error")) != "",
			extractLegacyContentText(payload["content"]),
			firstNonNil(payload["raw_output"], payload["rawOutput"], payload["raw"]),
		)
	}

	parsed.ToolError = nestedBool(payload, "tool_error") || strings.TrimSpace(nestedString(payload, "error")) != ""
	if parsed.ToolResult != nil && strings.TrimSpace(parsed.ToolResult.Error) != "" {
		parsed.ToolError = true
	}
	return parsed
}
