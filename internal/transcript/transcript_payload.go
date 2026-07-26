package transcript

import (
	"bytes"
	"encoding/json"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
)

func cloneToolResult(value *ToolResult) *ToolResult {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.StructuredPatch = acp.CloneRawMessage(value.StructuredPatch)
	cloned.RawOutput = acp.CloneRawMessage(value.RawOutput)
	return &cloned
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyRaw(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func rawToolResultOutput(value any) (json.RawMessage, map[string]any) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case json.RawMessage:
		return acp.CloneRawMessage(typed), nil
	case map[string]any:
		return rawMessageFromValue(typed), typed
	default:
		return rawMessageFromValue(value), nil
	}
}

func rawToolResultObject(raw json.RawMessage) map[string]any {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}

	var mapped map[string]any
	if err := json.Unmarshal(trimmed, &mapped); err != nil {
		return nil
	}
	return mapped
}

func canonicalPayload(
	eventType string,
	turnID string,
	timestamp time.Time,
	text string,
	toolName string,
	toolCallID string,
	toolInput json.RawMessage,
	toolResult *ToolResult,
	toolError bool,
) ([]byte, error) {
	payload := canonicalEventPayload{
		Schema:     CanonicalSchema,
		Type:       strings.TrimSpace(eventType),
		TurnID:     strings.TrimSpace(turnID),
		Timestamp:  timestamp.UTC(),
		Text:       text,
		ToolName:   toolName,
		ToolCallID: strings.TrimSpace(toolCallID),
		ToolInput:  acp.CloneRawMessage(toolInput),
		ToolResult: cloneToolResult(toolResult),
		ToolError:  toolError,
	}
	redactCanonicalPayload(&payload)

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("transcript: marshal canonical payload: %w", err)
	}
	return data, nil
}
