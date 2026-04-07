package agent

import (
	"strings"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	toolNameBash        = "Bash"
	toolNameClick       = "Click"
	toolNameDelete      = "Delete"
	toolNameEdit        = "Edit"
	toolNameFinance     = "Finance"
	toolNameFind        = "Find"
	toolNameGlob        = "Glob"
	toolNameGrep        = "Grep"
	toolNameImageSearch = "ImageSearch"
	toolNameOpenURL     = "OpenURL"
	toolNameRead        = "Read"
	toolNameSearch      = "Search"
	toolNameSports      = "Sports"
	toolNameTask        = "Task"
	toolNameThink       = "Think"
	toolNameTime        = "Time"
	toolNameTodoWrite   = "TodoWrite"
	toolNameToolCall    = "Tool Call"
	toolNameWeather     = "Weather"
	toolNameWebFetch    = "WebFetch"
	toolNameWebSearch   = "WebSearch"
	toolNameWrite       = "Write"
	emptyMapSentinel    = "map[]"
	emptyObjectLiteral  = "{}"
	jsonNullLiteral     = "null"
	toolNameMetaKey     = "tool_name"
)

var commonToolTitleAliases = map[string]string{
	"agent":           toolNameTask,
	"click":           toolNameClick,
	"codebase_search": toolNameGrep,
	"create_file":     toolNameWrite,
	"create_new_file": toolNameWrite,
	"edit_file":       toolNameEdit,
	"fd":              toolNameGlob,
	"file_search":     toolNameGlob,
	"finance":         toolNameFinance,
	"find":            toolNameFind,
	"glob":            toolNameGlob,
	"grep":            toolNameGrep,
	"grep_search":     toolNameGrep,
	"image_query":     toolNameImageSearch,
	"insert_text":     toolNameEdit,
	"list_dir":        toolNameRead,
	"open":            toolNameOpenURL,
	"read_file":       toolNameRead,
	"replace_in_file": toolNameEdit,
	"rg":              toolNameGrep,
	"ripgrep":         toolNameGrep,
	"run_subagent":    toolNameTask,
	"search_query":    toolNameWebSearch,
	"sports":          toolNameSports,
	"task":            toolNameTask,
	"time":            toolNameTime,
	"update_todo":     toolNameTodoWrite,
	"weather":         toolNameWeather,
	"web_search":      toolNameWebSearch,
	"write_file":      toolNameWrite,
	"write_to_file":   toolNameEdit,
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
		if token == "update_todo" || (input != nil && input["todos"] != nil) {
			return toolNameTodoWrite
		}
		return toolNameThink
	case acp.ToolKindSearch:
		switch token {
		case "glob", "fd", "file_search":
			return toolNameGlob
		case "find":
			return toolNameFind
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
		return toolNameDelete
	case acp.ToolKindExecute:
		return toolNameBash
	case acp.ToolKindSearch:
		if extractString(input, "pattern") != "" {
			return toolNameGrep
		}
		if looksLikeWebSearchInput(input) {
			return toolNameWebSearch
		}
		return toolNameSearch
	case acp.ToolKindFetch:
		if looksLikeWebSearchInput(input) {
			return toolNameWebSearch
		}
		return toolNameWebFetch
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
			return toolNameClick
		}
		if extractString(input, "pattern") != "" {
			return toolNameFind
		}
		return toolNameOpenURL
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
			return toolNameWebSearch, true
		case "image_query":
			return toolNameImageSearch, true
		}
	case model.IDEClaude, model.IDECursor, model.IDEDroid, model.IDEOpenCode, model.IDEPi, model.IDEGemini:
		// Use common aliases only.
	}
	return "", false
}

func looksLikeWebSearchInput(input map[string]any) bool {
	if input == nil {
		return false
	}
	for _, key := range []string{"query", "queries", "action_query", "action_type", "search_query", "image_query"} {
		if value, ok := input[key]; ok && value != nil {
			return true
		}
	}
	return false
}

func extractACPToolName(meta any) string {
	return extractString(coerceJSONObject(meta), toolNameMetaKey)
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	// Tool input normalization treats nested values as read-only and only needs
	// ownership of the top-level map keys.
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
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
