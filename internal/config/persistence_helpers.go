package config

import (
	"bytes"

	"fmt"
	"maps"

	"sort"
	"strings"

	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

func replaceOffsets(source []byte, start int, end int, replacement []byte) []byte {
	var buffer bytes.Buffer
	buffer.Grow(len(source) - (end - start) + len(replacement))
	buffer.Write(source[:start])
	buffer.Write(replacement)
	buffer.Write(source[end:])
	return buffer.Bytes()
}

func insertAtOffset(source []byte, offset int, fragment string) []byte {
	var buffer bytes.Buffer
	buffer.Grow(len(source) + len(fragment) + 1)
	buffer.Write(source[:offset])
	if offset > 0 && source[offset-1] != '\n' {
		buffer.WriteByte('\n')
	}
	buffer.WriteString(ensureTrailingNewline(fragment))
	buffer.Write(source[offset:])
	return buffer.Bytes()
}

func appendFragment(source []byte, fragment string) []byte {
	if len(source) == 0 {
		return []byte(ensureTrailingNewline(fragment))
	}

	var buffer bytes.Buffer
	buffer.Grow(len(source) + len(fragment) + 2)
	buffer.Write(source)
	if source[len(source)-1] != '\n' {
		buffer.WriteByte('\n')
	}
	if !bytes.HasSuffix(source, []byte("\n\n")) {
		buffer.WriteByte('\n')
	}
	buffer.WriteString(strings.TrimRight(fragment, "\n"))
	buffer.WriteByte('\n')
	return buffer.Bytes()
}

func ensureTrailingNewline(value string) string {
	if strings.HasSuffix(value, "\n") {
		return value
	}
	return value + "\n"
}

func rangeStart(r tomlast.Range) int {
	return int(r.Offset)
}

func rangeEnd(r tomlast.Range) int {
	return int(r.Offset + r.Length)
}

func clonePath(path []string) []string {
	if len(path) == 0 {
		return nil
	}
	return append([]string(nil), path...)
}

func pathsEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func pathHasPrefix(path []string, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for idx := range prefix {
		if path[idx] != prefix[idx] {
			return false
		}
	}
	return true
}

func unsupportedTOMLMutation(path []string, reason string) error {
	joined := strings.Join(path, ".")
	if strings.TrimSpace(joined) == "" {
		joined = "<root>"
	}
	return fmt.Errorf("%w at %q: %s", ErrUnsupportedTOMLMutation, joined, reason)
}

func sortedStringKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringMapToAny(values map[string]string) map[string]any {
	converted := make(map[string]any, len(values))
	for key, value := range values {
		converted[key] = value
	}
	return converted
}

func cloneStringAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	clone := make(map[string]any, len(values))
	maps.Copy(clone, values)
	return clone
}
