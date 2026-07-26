package transcript

import (
	"bytes"
	"encoding/json"

	"strings"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/store"
)

func buildToolResult(toolName string, failed bool, contentText string, rawOutput any) *ToolResult {
	result := &ToolResult{}

	displayText := strings.TrimSpace(firstNonEmpty(contentText, stringifyValue(rawOutput)))
	raw, mapped := rawToolResultOutput(rawOutput)
	if len(raw) > 0 {
		result.RawOutput = acp.CloneRawMessage(raw)
		if mapped == nil {
			mapped = rawToolResultObject(raw)
		}
		if mapped != nil {
			result.Stdout = firstNonEmpty(result.Stdout, nestedString(mapped, transcriptStdoutFieldKey))
			result.Stderr = firstNonEmpty(result.Stderr, nestedString(mapped, "stderr"))
			result.FilePath = firstNonEmpty(
				result.FilePath,
				nestedString(mapped, "file_path"),
				nestedString(mapped, "filePath"),
			)
			result.Content = firstNonEmpty(result.Content, nestedString(mapped, "content"))
			result.Error = firstNonEmpty(result.Error, nestedString(mapped, "error"))
			if patch := rawMessageFromValue(mapped["structuredPatch"]); len(patch) > 0 {
				result.StructuredPatch = acp.CloneRawMessage(patch)
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "bash":
		if failed {
			result.Stderr = firstNonEmpty(result.Stderr, displayText)
		} else {
			result.Stdout = firstNonEmpty(result.Stdout, displayText)
		}
	case "glob", "grep", "search":
		result.Stdout = firstNonEmpty(result.Stdout, displayText)
	case "read":
		result.Content = firstNonEmpty(result.Content, displayText)
	default:
		result.Content = firstNonEmpty(result.Content, displayText)
	}

	if failed {
		result.Error = firstNonEmpty(result.Error, displayText)
	}

	if result.Stdout == "" &&
		result.Stderr == "" &&
		result.FilePath == "" &&
		result.Content == "" &&
		len(result.StructuredPatch) == 0 &&
		result.Error == "" &&
		len(result.RawOutput) == 0 {
		return &ToolResult{}
	}

	return result
}

func decodeToolResult(raw json.RawMessage) *ToolResult {
	if len(raw) == 0 {
		return nil
	}
	var result ToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return &result
}

func sessionFailureFromValue(value any) *store.SessionFailure {
	raw := rawMessageFromValue(value)
	if len(raw) == 0 {
		return nil
	}
	var failure store.SessionFailure
	if err := json.Unmarshal(raw, &failure); err != nil {
		return nil
	}
	normalized := failure.Normalize()
	if normalized.IsZero() {
		return nil
	}
	return &normalized
}

func runtimeActivityFromValue(value any) *acp.RuntimeActivity {
	raw := rawMessageFromValue(value)
	if len(raw) == 0 {
		return nil
	}
	var activity acp.RuntimeActivity
	if err := json.Unmarshal(raw, &activity); err != nil {
		return nil
	}
	return cloneRuntimeActivity(&activity)
}

func extractLegacyContentText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case map[string]any:
		if text := nestedString(typed, "text"); strings.TrimSpace(text) != "" {
			return text
		}
		if inner, ok := typed["content"].(map[string]any); ok {
			return extractLegacyContentText(inner)
		}
		return ""
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(extractLegacyContentText(item))
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func legacyToolName(payload map[string]any) string {
	if meta, ok := payload["_meta"].(map[string]any); ok {
		for _, value := range meta {
			nested, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if toolName := strings.TrimSpace(nestedString(nested, "toolName")); toolName != "" {
				return toolName
			}
		}
	}
	return firstNonEmpty(nestedString(payload, "title"), nestedString(payload, "kind"))
}

func nestedString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func nestedBool(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	typed, ok := value.(bool)
	return ok && typed
}

func stringifyValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return extractLegacyContentText(value)
	}
}

func rawMessageFromValue(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}

func rawMessageIsEmptyObject(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("{}"))
}
