package transcript

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/acp"
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
