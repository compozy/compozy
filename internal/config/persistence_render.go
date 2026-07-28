package config

import (
	"fmt"

	"strings"

	tomltree "github.com/pelletier/go-toml"
	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

func renderBareValue(value any) (string, error) {
	tree, err := newEmptyTree()
	if err != nil {
		return "", err
	}
	normalized, err := normalizeTOMLValue(value)
	if err != nil {
		return "", err
	}
	tree.SetPath([]string{persistenceValueKey}, normalized)
	rendered, err := tree.ToTomlString()
	if err != nil {
		return "", fmt.Errorf("config: encode TOML value: %w", err)
	}
	_, after, ok := strings.Cut(rendered, "=")
	if !ok {
		return "", fmt.Errorf("config: encode TOML value: malformed fragment %q", rendered)
	}
	return strings.TrimSpace(after), nil
}

func isUsableValueRange(raw tomlast.Range, value tomlast.Range) bool {
	if value.Length == 0 {
		return false
	}
	rawStart := rangeStart(raw)
	rawEnd := rangeEnd(raw)
	valueStart := rangeStart(value)
	valueEnd := rangeEnd(value)
	return valueStart >= rawStart && valueEnd <= rawEnd && valueStart < valueEnd
}

func lineReplacementOffsets(source []byte, target tomlast.Range) (int, int) {
	start := min(max(rangeStart(target), 0), len(source))

	end := start
	for end < len(source) && source[end] != '\n' {
		end++
	}
	if end < len(source) {
		end++
	}

	return start, end
}

func lineEndOffset(source []byte, target tomlast.Range) int {
	_, end := lineReplacementOffsets(source, target)
	return end
}

func renderKeyValueLine(path []string, value any) (string, error) {
	tree, err := newEmptyTree()
	if err != nil {
		return "", err
	}
	normalized, err := normalizeTOMLValue(value)
	if err != nil {
		return "", err
	}
	tree.SetPath(path, normalized)
	rendered, err := tree.ToTomlString()
	if err != nil {
		return "", fmt.Errorf("config: encode TOML key-value: %w", err)
	}
	return ensureTrailingNewline(strings.TrimRight(rendered, "\n")), nil
}

func renderTableFragment(path []string, values map[string]any) (string, error) {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return "", err
	}
	body, err := renderTreeFragment(values)
	if err != nil {
		return "", err
	}

	lines := []string{"[" + strings.Join(cleanPath, ".") + "]"}
	for line := range strings.SplitSeq(strings.TrimRight(body, "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]"):
			nested := strings.TrimSuffix(strings.TrimPrefix(trimmed, "[["), "]]")
			lines = append(lines, "[["+strings.Join(append(clonePath(cleanPath), nested), ".")+"]]")
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			nested := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
			lines = append(lines, "["+strings.Join(append(clonePath(cleanPath), nested), ".")+"]")
		default:
			lines = append(lines, trimmed)
		}
	}

	return ensureTrailingNewline(strings.Join(lines, "\n")), nil
}

func renderArrayTableFragment(path []string, values map[string]any) (string, error) {
	cleanPath, err := normalizeMutationPath(path)
	if err != nil {
		return "", err
	}
	body, err := renderTreeFragment(values)
	if err != nil {
		return "", err
	}
	header := "[[" + strings.Join(cleanPath, ".") + "]]"

	lines := []string{header}
	for line := range strings.SplitSeq(strings.TrimRight(body, "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]"):
			nested := strings.TrimSuffix(strings.TrimPrefix(trimmed, "[["), "]]")
			lines = append(lines, "[["+strings.Join(append(clonePath(cleanPath), nested), ".")+"]]")
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			nested := strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]")
			lines = append(lines, "["+strings.Join(append(clonePath(cleanPath), nested), ".")+"]")
		default:
			lines = append(lines, trimmed)
		}
	}

	return ensureTrailingNewline(strings.Join(lines, "\n")), nil
}

func renderTreeFragment(values map[string]any) (string, error) {
	tree, err := newTreeFromMap(values)
	if err != nil {
		return "", err
	}
	rendered, err := tree.ToTomlString()
	if err != nil {
		return "", fmt.Errorf("config: encode TOML fragment: %w", err)
	}
	return rendered, nil
}

func decodeStringValue(value []byte) (string, bool) {
	tree, err := tomltree.LoadBytes([]byte("value = " + strings.TrimSpace(string(value))))
	if err != nil {
		return "", false
	}
	text, ok := tree.Get(persistenceValueKey).(string)
	return text, ok
}

func replaceRange(source []byte, r tomlast.Range, replacement []byte) []byte {
	return replaceOffsets(source, rangeStart(r), rangeEnd(r), replacement)
}
