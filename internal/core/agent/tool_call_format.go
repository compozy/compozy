package agent

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	toolNameBash       = "Bash"
	toolNameEdit       = "Edit"
	toolNameGrep       = "Grep"
	toolNameRead       = "Read"
	toolNameTask       = "Task"
	toolNameTodoWrite  = "TodoWrite"
	toolNameToolCall   = "Tool Call"
	toolNameWebFetch   = "WebFetch"
	toolNameWebSearch  = "WebSearch"
	emptyMapSentinel   = "map[]"
	emptyObjectLiteral = "{}"
	jsonNullLiteral    = "null"
	toolNameMetaKey    = "tool_name"
)

var commonToolTitleAliases = map[string]string{
	"agent":           "Task",
	"click":           "Click",
	"codebase_search": "Grep",
	"create_file":     "Write",
	"create_new_file": "Write",
	"edit_file":       "Edit",
	"fd":              "Glob",
	"file_search":     "Glob",
	"finance":         "Finance",
	"find":            "Find",
	"glob":            "Glob",
	"grep":            "Grep",
	"grep_search":     "Grep",
	"image_query":     "ImageSearch",
	"insert_text":     "Edit",
	"list_dir":        "Read",
	"open":            "OpenURL",
	"read_file":       "Read",
	"replace_in_file": "Edit",
	"rg":              "Grep",
	"ripgrep":         "Grep",
	"run_subagent":    "Task",
	"search_query":    "WebSearch",
	"sports":          "Sports",
	"task":            "Task",
	"time":            "Time",
	"update_todo":     "TodoWrite",
	"weather":         "Weather",
	"web_search":      "WebSearch",
	"write_file":      "Write",
	"write_to_file":   "Edit",
}

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
	nameHint := title
	if strings.TrimSpace(nameHint) == "" {
		nameHint = metaToolName
	}

	normalizedInput := normalizeACPToolInput(driverID, nameHint, kind, rawInput, locations)
	name := normalizeACPToolName(driverID, nameHint, kind, normalizedInput)

	inputPayload := marshalRawJSON(normalizedInput)
	if len(inputPayload) == 0 {
		inputPayload = marshalRawJSON(rawInput)
	}
	inputPayload = sanitizeToolUseInputPayload(inputPayload)
	rawInputPayload := sanitizeToolUseInputPayload(marshalRawJSON(rawInput))

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

func normalizeACPToolName(
	driverID string,
	title string,
	kind acp.ToolKind,
	input map[string]any,
) string {
	token := canonicalToolToken(title)
	if name := normalizeToolNameByKind(token, kind, input); name != "" {
		return name
	}
	if inferred := inferToolNameFromInputShape(input); inferred != "" {
		return inferred
	}

	if alias, ok := driverToolTitleAlias(driverID, token); ok {
		return alias
	}
	if alias, ok := commonToolTitleAliases[token]; ok {
		return alias
	}
	if name := normalizeToolNameFallback(kind, input, title); name != "" {
		return name
	}
	return toolNameToolCall
}

func normalizeToolNameByKind(token string, kind acp.ToolKind, input map[string]any) string {
	switch kind {
	case acp.ToolKindThink:
		if token == "update_todo" || input != nil && input["todos"] != nil {
			return toolNameTodoWrite
		}
		return "Think"
	case acp.ToolKindSearch:
		switch token {
		case "glob", "find", "fd", "file_search":
			return "Glob"
		case "grep", "rg", "ripgrep", "codebase_search", "grep_search":
			return toolNameGrep
		}
		if looksLikeWebSearchInput(input) {
			return toolNameWebSearch
		}
	}
	return ""
}

func normalizeToolNameFallback(kind acp.ToolKind, input map[string]any, title string) string {
	switch kind {
	case acp.ToolKindRead:
		return toolNameRead
	case acp.ToolKindEdit:
		return toolNameEdit
	case acp.ToolKindDelete:
		return "Delete"
	case acp.ToolKindExecute:
		return toolNameBash
	case acp.ToolKindSearch:
		if extractString(input, "pattern") != "" {
			return toolNameGrep
		}
		if looksLikeWebSearchInput(input) {
			return toolNameWebSearch
		}
		return "Search"
	case acp.ToolKindFetch:
		if looksLikeWebSearchInput(input) {
			return toolNameWebSearch
		}
		return toolNameWebFetch
	case acp.ToolKindOther:
		if trimmed := strings.TrimSpace(title); trimmed != "" {
			return trimmed
		}
	}
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed
	}
	return ""
}

func inferToolNameFromInputShape(input map[string]any) string {
	if input == nil {
		return ""
	}
	if extractString(input, "task", "prompt", "description") != "" {
		return toolNameTask
	}
	if input["todos"] != nil {
		return toolNameTodoWrite
	}
	if extractString(input, "command") != "" {
		return toolNameBash
	}
	if looksLikeWebSearchInput(input) {
		return toolNameWebSearch
	}
	if refID := extractString(input, "ref_id", "refId"); refID != "" {
		if _, ok := extractInt(input, "id"); ok {
			return "Click"
		}
		if extractString(input, "pattern") != "" {
			return "Find"
		}
		if extractString(input, "url") != "" || refID != "" {
			return "OpenURL"
		}
	}
	if extractString(input, "url") != "" {
		return toolNameWebFetch
	}
	if extractString(input, "pattern") != "" {
		return toolNameGrep
	}
	if extractString(input, "file_path", "filePath", "path", "notebook_path", "notebookPath") != "" {
		if extractString(
			input,
			"old_string",
			"oldString",
			"new_string",
			"newString",
			"content",
			"new_text",
			"newText",
		) != "" {
			return toolNameEdit
		}
		return toolNameRead
	}
	return ""
}

func driverToolTitleAlias(driverID string, token string) (string, bool) {
	switch driverID {
	case model.IDECodex:
		switch token {
		case "search_query":
			return "WebSearch", true
		case "image_query":
			return "ImageSearch", true
		}
	case model.IDEClaude, model.IDECursor, model.IDEDroid, model.IDEOpenCode, model.IDEPi, model.IDEGemini:
		// Use common aliases only.
	}
	return "", false
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
	case "Glob":
		normalizeGlobToolInput(normalized, rawInput, raw)
	case toolNameWebFetch, "OpenURL":
		normalizeOpenURLInput(normalized, raw)
	case toolNameWebSearch, "ImageSearch":
		mergeWebSearchInput(normalized, raw, title)
	case "Click":
		normalizeClickInput(normalized, raw)
	case "Find":
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
	case toolNameRead, "Write", toolNameEdit, "Delete":
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

func looksLikeWebSearchInput(input map[string]any) bool {
	if input == nil {
		return false
	}
	for _, key := range []string{"query", "queries", "action_query", "action_type", "url", "search_query", "image_query"} {
		if value, ok := input[key]; ok && value != nil {
			return true
		}
	}
	return false
}

func canonicalToolToken(title string) string {
	token := strings.TrimSpace(strings.ToLower(title))
	if token == "" {
		return ""
	}
	if strings.HasPrefix(token, "mcp__") {
		parts := strings.Split(token, "__")
		token = parts[len(parts)-1]
	}
	if strings.Contains(token, ".") {
		parts := strings.Split(token, ".")
		token = parts[len(parts)-1]
	}
	return token
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

func extractACPToolName(meta any) string {
	return extractString(coerceJSONObject(meta), toolNameMetaKey)
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
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
