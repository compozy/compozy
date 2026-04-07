package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func buildNormalizedToolUseBlock(
	driverID string,
	toolCallID string,
	title string,
	kind acp.ToolKind,
	rawInput any,
	locations []acp.ToolCallLocation,
	meta any,
) (model.ContentBlock, error) {
	metaToolName := extractACPToolName(meta)
	nameHint := metaToolName
	if strings.TrimSpace(nameHint) == "" {
		nameHint = title
	}

	normalizedInput := normalizeACPToolInput(driverID, nameHint, kind, rawInput, locations)
	name := normalizeACPToolName(driverID, nameHint, kind, normalizedInput)

	inputPayload, err := marshalRawJSON(normalizedInput)
	if err != nil {
		return model.ContentBlock{}, fmt.Errorf("marshal normalized tool input: %w", err)
	}
	if len(inputPayload) == 0 {
		inputPayload, err = marshalRawJSON(rawInput)
		if err != nil {
			return model.ContentBlock{}, fmt.Errorf("marshal fallback tool input: %w", err)
		}
	}
	inputPayload = sanitizeToolUseInputPayload(inputPayload)
	rawInputPayload, err := marshalRawJSON(rawInput)
	if err != nil {
		return model.ContentBlock{}, fmt.Errorf("marshal raw tool input: %w", err)
	}
	rawInputPayload = sanitizeToolUseInputPayload(rawInputPayload)

	displayTitle := ""
	if meaningfulToolHeaderTitle(title) {
		displayTitle = strings.TrimSpace(title)
	}

	return model.NewContentBlock(model.ToolUseBlock{
		ID:       toolCallID,
		Name:     name,
		Title:    displayTitle,
		ToolName: metaToolName,
		Input:    inputPayload,
		RawInput: rawInputPayload,
	})
}

func sanitizeToolUseInputPayload(payload json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" || trimmed == jsonNullLiteral {
		return nil
	}
	return payload
}

func normalizeACPToolInput(
	driverID string,
	title string,
	kind acp.ToolKind,
	rawInput any,
	locations []acp.ToolCallLocation,
) map[string]any {
	raw := coerceJSONObject(rawInput)
	name := normalizeACPToolName(driverID, title, kind, raw)
	normalized := normalizeToolInputByName(name, title, rawInput, raw, locations)
	return finalizeToolInput(normalized, raw, rawInput)
}

func normalizeToolInputByName(
	name string,
	title string,
	rawInput any,
	raw map[string]any,
	locations []acp.ToolCallLocation,
) map[string]any {
	normalized := make(map[string]any)
	if isFileToolName(name) {
		normalizeFileToolInput(normalized, raw, locations)
		return normalized
	}

	switch name {
	case toolNameBash:
		normalizeBashToolInput(normalized, rawInput, raw)
	case toolNameGrep:
		normalizeGrepToolInput(normalized, rawInput, raw)
	case toolNameGlob:
		normalizeGlobToolInput(normalized, rawInput, raw)
	case toolNameWebFetch, toolNameOpenURL:
		normalizeOpenURLInput(normalized, raw)
	case toolNameWebSearch, toolNameImageSearch:
		mergeWebSearchInput(normalized, raw, title)
	case toolNameClick:
		normalizeClickInput(normalized, raw)
	case toolNameFind:
		normalizeFindInput(normalized, raw)
	case toolNameTask:
		normalizeTaskToolInput(normalized, raw)
	case toolNameTodoWrite:
		if todos := raw["todos"]; todos != nil {
			normalized["todos"] = todos
		}
	default:
		if !normalizeCollectionToolInput(normalized, raw, name) {
			normalizeFallbackToolInput(normalized, rawInput)
		}
	}
	return normalized
}

func isFileToolName(name string) bool {
	switch name {
	case toolNameRead, toolNameWrite, toolNameEdit, toolNameDelete:
		return true
	default:
		return false
	}
}

func normalizeFileToolInput(
	normalized map[string]any,
	raw map[string]any,
	locations []acp.ToolCallLocation,
) {
	if path := extractToolPath(raw, locations); path != "" {
		normalized["file_path"] = path
	}
	if startLine, ok := extractInt(raw, "start_line", "startLine", "startLineNumberBaseOne"); ok {
		normalized["start_line"] = startLine
	}
	if endLine, ok := extractInt(raw, "end_line", "endLine", "endLineNumberBaseOne"); ok {
		normalized["end_line"] = endLine
	}
	if content := extractString(raw, "content", "new_text", "newText"); content != "" {
		normalized["content"] = content
	}
	if oldString := extractString(raw, "old_string", "oldString"); oldString != "" {
		normalized["old_string"] = oldString
	}
	if newString := extractString(raw, "new_string", "newString"); newString != "" {
		normalized["new_string"] = newString
	}
}

func normalizeBashToolInput(normalized map[string]any, rawInput any, raw map[string]any) {
	if command := extractShellCommandValue(rawInput); command != "" {
		normalized["command"] = command
	}
	if cwd := extractString(raw, "cwd"); cwd != "" {
		normalized["cwd"] = cwd
	}
}

func normalizeGrepToolInput(normalized map[string]any, rawInput any, raw map[string]any) {
	if pattern := extractString(raw, "pattern", "query", "q"); pattern != "" {
		normalized["pattern"] = pattern
	}
	if path := extractString(raw, "path", "cwd"); path != "" {
		normalized["path"] = path
	}
	if glob := extractString(raw, "glob", "includePattern"); glob != "" {
		normalized["glob"] = glob
	}
	if len(normalized) == 0 {
		normalizeFallbackToolInput(normalized, rawInput)
	}
}

func normalizeGlobToolInput(normalized map[string]any, rawInput any, raw map[string]any) {
	if pattern := extractString(raw, "pattern", "path", "glob"); pattern != "" {
		normalized["pattern"] = pattern
	}
	if path := extractString(raw, "cwd"); path != "" {
		normalized["path"] = path
	}
	if len(normalized) == 0 {
		normalizeFallbackToolInput(normalized, rawInput)
	}
}

func normalizeOpenURLInput(normalized map[string]any, raw map[string]any) {
	if url := extractString(raw, "url"); url != "" {
		normalized["url"] = url
	}
	if refID := extractString(raw, "ref_id", "refId"); refID != "" {
		normalized["ref_id"] = refID
	}
}

func normalizeClickInput(normalized map[string]any, raw map[string]any) {
	if refID := extractString(raw, "ref_id", "refId"); refID != "" {
		normalized["ref_id"] = refID
	}
	if id, ok := extractInt(raw, "id"); ok {
		normalized["id"] = id
	}
}

func normalizeFindInput(normalized map[string]any, raw map[string]any) {
	if refID := extractString(raw, "ref_id", "refId"); refID != "" {
		normalized["ref_id"] = refID
	}
	if pattern := extractString(raw, "pattern"); pattern != "" {
		normalized["pattern"] = pattern
	}
}

func normalizeTaskToolInput(normalized map[string]any, raw map[string]any) {
	if subagentType := extractString(raw, "agentName", "agent_name", "subagent_type"); subagentType != "" {
		normalized["subagent_type"] = subagentType
	}
	if prompt := extractString(raw, "task", "prompt", "description"); prompt != "" {
		normalized["prompt"] = prompt
	}
}

func normalizeFallbackToolInput(normalized map[string]any, rawInput any) {
	if command := extractShellCommandValue(rawInput); command != "" {
		normalized["command"] = command
	}
}

func normalizeCollectionToolInput(normalized map[string]any, raw map[string]any, name string) bool {
	switch name {
	case "Finance":
		mergeFirstObjectFields(normalized, raw, "finance", []string{"ticker", "type", "market"})
	case "Weather":
		mergeFirstObjectFields(normalized, raw, "weather", []string{"location", "start", "duration"})
	case "Sports":
		mergeFirstObjectFields(
			normalized,
			raw,
			"sports",
			[]string{"fn", "league", "team", "opponent", "date_from", "date_to"},
		)
	case "Time":
		mergeFirstObjectFields(normalized, raw, "time", []string{"utc_offset"})
	default:
		return false
	}
	return true
}

func finalizeToolInput(normalized map[string]any, raw map[string]any, rawInput any) map[string]any {
	if len(normalized) > 0 {
		return normalized
	}
	if len(raw) > 0 {
		return raw
	}
	text := stringifyToolInputValue(rawInput)
	if text == "" || text == "<nil>" || text == jsonNullLiteral || text == emptyObjectLiteral ||
		text == emptyMapSentinel {
		return nil
	}
	return map[string]any{"value": text}
}

func stringifyToolInputValue(rawInput any) string {
	switch typed := rawInput.(type) {
	case nil:
		return ""
	case json.RawMessage:
		return strings.TrimSpace(string(typed))
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(rawInput))
	}
}

func mergeWebSearchInput(normalized map[string]any, raw map[string]any, title string) {
	queries := extractQueryList(raw, title)
	if len(queries) > 0 {
		normalized["queries"] = queries
		if _, ok := normalized["query"]; !ok {
			normalized["query"] = queries[0]
		}
	}
	if query := extractString(raw, "query", "q"); query != "" {
		normalized["query"] = query
	}
	if actionQuery := extractString(raw, "action_query", "actionQuery"); actionQuery != "" {
		normalized["action_query"] = actionQuery
	}
	if actionType := extractString(raw, "action_type", "actionType", "type"); actionType != "" {
		normalized["action_type"] = actionType
	}
	if url := extractString(raw, "url"); url != "" {
		normalized["url"] = url
	}
	if pattern := extractString(raw, "pattern"); pattern != "" {
		normalized["pattern"] = pattern
	}
	if refID := extractString(raw, "ref_id", "refId"); refID != "" {
		normalized["ref_id"] = refID
	}
	if responseLength := extractString(raw, "response_length", "responseLength"); responseLength != "" {
		normalized["response_length"] = responseLength
	}
}

func mergeFirstObjectFields(dst map[string]any, raw map[string]any, key string, fields []string) {
	if len(dst) != 0 || raw == nil {
		return
	}

	object := firstObjectFromList(raw[key])
	if object == nil {
		object = raw
	}
	for _, field := range fields {
		if value, ok := object[field]; ok && value != nil {
			dst[field] = value
		}
	}
}

func firstObjectFromList(value any) map[string]any {
	list, ok := value.([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	record, ok := list[0].(map[string]any)
	if !ok {
		return nil
	}
	return record
}

func extractToolPath(raw map[string]any, locations []acp.ToolCallLocation) string {
	if len(locations) > 0 {
		for _, location := range locations {
			if strings.TrimSpace(location.Path) != "" {
				return strings.TrimSpace(location.Path)
			}
		}
	}
	return extractString(raw, "file_path", "filePath", "path", "notebook_path", "notebookPath")
}

func extractQueryList(raw map[string]any, title string) []string {
	var keys []string
	switch canonicalToolToken(title) {
	case "image_query":
		keys = []string{"image_query"}
	default:
		keys = []string{"search_query", "queries"}
	}

	for _, key := range keys {
		switch value := raw[key].(type) {
		case []string:
			return append([]string(nil), value...)
		case []map[string]any:
			queries := make([]string, 0, len(value))
			for _, item := range value {
				if query := extractString(item, "q", "query"); query != "" {
					queries = append(queries, query)
				}
			}
			if len(queries) > 0 {
				return queries
			}
		case []any:
			queries := make([]string, 0, len(value))
			for _, item := range value {
				switch typed := item.(type) {
				case string:
					if trimmed := strings.TrimSpace(typed); trimmed != "" {
						queries = append(queries, trimmed)
					}
				case map[string]any:
					if query := extractString(typed, "q", "query"); query != "" {
						queries = append(queries, query)
					}
				}
			}
			if len(queries) > 0 {
				return queries
			}
		}
	}
	return nil
}

func coerceJSONObject(value any) map[string]any {
	switch typed := value.(type) {
	case nil:
		return nil
	case map[string]any:
		return cloneMap(typed)
	case json.RawMessage:
		if len(typed) == 0 {
			return nil
		}
		var record map[string]any
		if err := json.Unmarshal(typed, &record); err == nil {
			return record
		}
		return nil
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var record map[string]any
		if err := json.Unmarshal(payload, &record); err != nil {
			return nil
		}
		return record
	}
}

func extractString(raw map[string]any, keys ...string) string {
	if raw == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := raw[key].(string)
		if ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func extractInt(raw map[string]any, keys ...string) (int, bool) {
	if raw == nil {
		return 0, false
	}
	for _, key := range keys {
		switch value := raw[key].(type) {
		case int:
			return value, true
		case int32:
			return int(value), true
		case int64:
			return int(value), true
		case float64:
			return int(value), true
		case json.Number:
			parsed, err := value.Int64()
			if err == nil {
				return int(parsed), true
			}
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func extractShellCommandValue(rawInput any) string {
	switch value := rawInput.(type) {
	case string:
		return strings.TrimSpace(value)
	case []string:
		if len(value) == 0 {
			return ""
		}
		return strings.TrimSpace(value[len(value)-1])
	case []any:
		if len(value) == 0 {
			return ""
		}
		last, ok := value[len(value)-1].(string)
		if !ok {
			return ""
		}
		return strings.TrimSpace(last)
	case map[string]any:
		if command, ok := value["command"]; ok {
			return extractShellCommandValue(command)
		}
	}
	return ""
}
