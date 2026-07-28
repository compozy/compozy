package acp

import (
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

func permissionRequestIDFromMeta(meta any) string {
	record, ok := meta.(map[string]any)
	if !ok {
		return ""
	}

	for _, key := range []string{permissionRequestIDKey, "requestId", "id"} {
		if value, ok := record[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func permissionRequestName(turnID string, request acpsdk.RequestPermissionRequest) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(turnID); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if title := toolCallTitle(request.ToolCall); title != "" {
		parts = append(parts, title)
	} else if kind := toolCallKind(request.ToolCall); kind != "" {
		parts = append(parts, kind)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ":")
}

func toolCallTitle(toolCall acpsdk.ToolCallUpdate) string {
	if toolCall.Title == nil {
		return ""
	}
	return strings.TrimSpace(*toolCall.Title)
}

func toolCallKind(toolCall acpsdk.ToolCallUpdate) string {
	if toolCall.Kind == nil {
		return ""
	}
	return strings.TrimSpace(string(*toolCall.Kind))
}
