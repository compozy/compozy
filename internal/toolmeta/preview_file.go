package toolmeta

import (
	"fmt"
	"path"
	"strings"
)

func filePreview(input map[string]any) string {
	path := scalarString(input["file_path"])
	if path == "" {
		path = scalarString(input["path"])
	}
	name := portableFileBasename(path)
	start, hasStart := numericInput(input, "start_line", "line_start", "offset")
	end, hasEnd := numericInput(input, "end_line", "line_end")
	switch {
	case name == "":
		return ""
	case hasStart && hasEnd:
		return fmt.Sprintf("%s:%d-%d", name, start, end)
	case hasStart:
		return fmt.Sprintf("%s:%d", name, start)
	default:
		return name
	}
}

func portableFileBasename(value string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), `\`, "/")
	if normalized == "" {
		return ""
	}
	if hasWindowsDrivePrefix(normalized) {
		normalized = normalized[2:]
	}
	if strings.HasPrefix(normalized, "//") {
		parts := nonEmptyPathParts(normalized)
		if len(parts) <= 2 {
			return ""
		}
		normalized = parts[len(parts)-1]
	}
	normalized = strings.TrimRight(normalized, "/")
	if normalized == "" {
		return ""
	}
	name := path.Base(normalized)
	if name == "." || name == ".." || name == "/" {
		return ""
	}
	return name
}

func hasWindowsDrivePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	letter := value[0]
	return letter >= 'A' && letter <= 'Z' || letter >= 'a' && letter <= 'z'
}

func nonEmptyPathParts(value string) []string {
	parts := strings.Split(value, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func numericInput(input map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := input[key]
		if !ok {
			continue
		}
		number, ok := value.(float64)
		if ok {
			return int(number), true
		}
	}
	return 0, false
}
