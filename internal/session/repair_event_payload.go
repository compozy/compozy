package session

import (
	"encoding/json"
	"fmt"
	"strings"
)

func interruptedToolResultRaw(toolCallID string, toolName string) (json.RawMessage, error) {
	metadata := map[string]any{
		"agh": map[string]any{
			"repair":   true,
			"toolName": strings.TrimSpace(toolName),
		},
	}
	payload := map[string]any{
		"sessionUpdate": "tool_call_update",
		repairStatusKey: "failed",
		"toolCallId":    strings.TrimSpace(toolCallID),
		"rawOutput": map[string]string{
			"stderr": repairInterruptedToolMessage,
			"error":  repairInterruptedToolMessage,
		},
		repairContentKey: []map[string]any{
			{
				repairTypeKey: repairContentKey,
				repairContentKey: map[string]string{
					repairTypeKey:            hookMessageDeltaTypeText,
					hookMessageDeltaTypeText: repairInterruptedToolMessage,
				},
			},
		},
		"_meta": metadata,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("session: marshal interrupted tool result: %w", err)
	}
	return data, nil
}
