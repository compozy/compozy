package soul

import (
	"errors"
	"fmt"

	"strings"
	"unicode"
)

func isAllowedField(key string) bool {
	switch key {
	case soulVersionKey,
		soulRoleKey,
		soulToneKey,
		"principles",
		"constraints",
		"collaboration",
		"memory_policy",
		"tags":
		return true
	default:
		return false
	}
}

func forbiddenOwner(key string) string {
	switch key {
	case "name", "provider", "command", "model", "tools", "toolsets", "deny_tools", "permissions",
		"mcp_servers", "hooks", "prompt":
		return "AGENT.md"
	case soulCapabilitiesKey, "capability":
		return soulCapabilitiesKey
	case "tasks", "task", soulTaskRunsKey, "run", "ownership", "claim_token", "claim_token_hash":
		return "task runtime"
	case "scheduler", "heartbeat", "lease", "session", "session_liveness", "activity", "wake":
		return "runtime state"
	case "network", "channels", "peers", "presence":
		return "AGH Network presence"
	case "spawn":
		return "session spawn overlays"
	case "env", soulConfigKey, "defaults", "providers", "sandboxes", "settings":
		return soulConfigKey
	case "memory", "memory_store", "memory_scope", "memory_type", "memories":
		return "memory runtime"
	default:
		return ""
	}
}

func markdownHeadingKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level >= len(trimmed) || !unicode.IsSpace(rune(trimmed[level])) {
		return "", false
	}
	title := strings.TrimSpace(trimmed[level:])
	title = strings.Trim(title, "# ")
	if title == "" {
		return "", false
	}
	return normalizeKey(title), true
}

func normalizeKey(value string) string {
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func frontmatterKeyLocation(metadata []byte, key string) (int, int) {
	lines := strings.Split(string(metadata), "\n")
	for idx, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		col := len(line) - len(trimmed) + 1
		before, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		candidate := strings.Trim(strings.TrimSpace(before), `"'`)
		if candidate == key {
			return idx + 2, col
		}
	}
	return 2, 1
}

func bodyStartLine(metadata []byte) int {
	if len(metadata) == 0 {
		return 3
	}
	return 3 + strings.Count(string(metadata), "\n")
}

func normalizeLineEndings(content []byte) []byte {
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return []byte(normalized)
}

func normalizeBody(body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.TrimRightFunc(normalized, unicode.IsSpace)
}

func stringOnly(value any) (string, error) {
	typed, ok := value.(string)
	if !ok {
		return "", errors.New("value is not a string")
	}
	return strings.TrimSpace(typed), nil
}

func scalarString(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), nil
	case int:
		return fmt.Sprintf("%d", typed), nil
	case int64:
		return fmt.Sprintf("%d", typed), nil
	case uint64:
		return fmt.Sprintf("%d", typed), nil
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%.0f", typed), nil
		}
	}
	return "", errors.New("value is not a scalar string")
}

func stringList(value any, key string) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, nil
		}
		return []string{trimmed}, nil
	case []string:
		return normalizeStrings(typed), nil
	case []any:
		values := make([]string, 0, len(typed))
		for idx, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("SOUL.md frontmatter field %q item %d must be a string", key, idx)
			}
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				values = append(values, trimmed)
			}
		}
		return values, nil
	default:
		return nil, fmt.Errorf("SOUL.md frontmatter field %q must be a string list", key)
	}
}

func normalizeStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}
