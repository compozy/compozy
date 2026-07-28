package extensionpkg_test

import (
	"bytes"
	"encoding/json"
	"strings"
)

func nonEmptyLines(input string) []string {
	lines := strings.Split(input, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func containsFragmentsInOrder(text string, fragments ...string) bool {
	searchFrom := 0
	for _, fragment := range fragments {
		if fragment == "" {
			continue
		}
		offset := strings.Index(text[searchFrom:], fragment)
		if offset < 0 {
			return false
		}
		searchFrom += offset + len(fragment)
	}
	return true
}

func decodeJSONLines[T any](payload []byte) ([]T, error) {
	lines := nonEmptyLines(string(payload))
	decoded := make([]T, 0, len(lines))
	for _, line := range lines {
		var item T
		if err := json.NewDecoder(bytes.NewBufferString(line)).Decode(&item); err != nil {
			return nil, err
		}
		decoded = append(decoded, item)
	}
	return decoded, nil
}
