package transcript

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func toolInputFromDecoded(decoded *decodedStoredEvent) (json.RawMessage, bool) {
	if decoded == nil {
		return nil, false
	}
	if len(decoded.parsed.ToolInput) == 0 {
		return nil, false
	}
	var value any
	if err := json.Unmarshal(decoded.parsed.ToolInput, &value); err != nil {
		return acp.CloneRawMessage(decoded.parsed.ToolInput), true
	}
	if !toolInputReadyValue(value) {
		return nil, false
	}
	return acp.CloneRawMessage(decoded.parsed.ToolInput), true
}

func toolInputReadyValue(input any) bool {
	switch value := input.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(value) != ""
	case []any:
		return len(value) > 0
	case map[string]any:
		return len(value) > 0
	default:
		return true
	}
}

func inputMessageID(decoded *decodedStoredEvent, role string) string {
	suffix := role
	if role == UIRoleSystem {
		suffix = "system"
	}
	return fallbackMessageID(strings.TrimSpace(decoded.stored.ID), strings.TrimSpace(decoded.parsed.TurnID), suffix)
}

func assistantMessageID(decoded *decodedStoredEvent) string {
	if decoded != nil && decoded.parsed.Type == acp.EventTypeError {
		return fallbackMessageID(
			strings.TrimSpace(decoded.stored.ID),
			strings.TrimSpace(decoded.parsed.TurnID),
			"assistant-error",
		)
	}
	return fallbackMessageID(
		strings.TrimSpace(decoded.parsed.TurnID),
		strings.TrimSpace(decoded.stored.ID),
		"assistant",
	)
}

func fallbackMessageID(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return fmt.Sprintf("msg-%s", CanonicalSchema)
}

func uniqueUIMessageID(id string, used map[string]struct{}) string {
	base := fallbackMessageID(id, "message")
	if _, ok := used[base]; !ok {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}
