package core

import (
	"encoding/json"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func (e *PromptStreamEncoder) toolNameByID(toolCallID string) string {
	toolName := strings.TrimSpace(e.toolNames[toolCallID])
	if toolName == "" {
		return "tool"
	}
	return toolName
}

func (e *PromptStreamEncoder) toolName(event acp.AgentEvent) string {
	if toolName := strings.TrimSpace(event.ToolName()); toolName != "" {
		return toolName
	}
	rawPayload := promptRawEventMap(event.Raw)
	if toolName := promptMetaToolName(rawPayload); toolName != "" {
		return toolName
	}
	if toolName := strings.TrimSpace(promptStringValue(rawPayload["tool_name"])); toolName != "" {
		return toolName
	}
	return strings.TrimSpace(event.Title)
}

func promptMetaToolName(rawPayload map[string]any) string {
	meta := promptMapValue(rawPayload["_meta"])
	if meta == nil {
		return ""
	}
	for _, value := range meta {
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if toolName := strings.TrimSpace(promptStringValue(nested["toolName"])); toolName != "" {
			return toolName
		}
	}
	return ""
}

func promptNormalizedToolInput(event acp.AgentEvent) (any, bool) {
	if typedInput := event.ToolInput(); len(typedInput) > 0 {
		var input any
		if err := json.Unmarshal(typedInput, &input); err == nil && input != nil {
			return promptRedactValue(input), true
		}
	}

	rawPayload := promptRawEventMap(event.Raw)
	if len(rawPayload) == 0 {
		return nil, false
	}

	input, ok := promptFirstNonNil(
		rawPayload["tool_input"],
		rawPayload["rawInput"],
	)
	if !ok || input == nil {
		return nil, false
	}

	return promptRedactValue(input), true
}

func promptRawEventMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}
