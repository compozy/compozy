package agent

import (
	"strings"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	toolNameBash       = "Bash"
	toolNameEdit       = "Edit"
	toolNameFind       = "Find"
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
	"find":            toolNameFind,
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
		case "glob", "fd", "file_search":
			return "Glob"
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
			return toolNameFind
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
