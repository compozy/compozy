package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func normalizeRawJSON(value json.RawMessage, label string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("bridges: %s must be valid JSON", label)
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, trimmed); err != nil {
		return nil, fmt.Errorf("bridges: compact %s: %w", label, err)
	}
	return compacted.Bytes(), nil
}

func normalizeJSONObject(value json.RawMessage, label string) (json.RawMessage, error) {
	normalized, err := normalizeRawJSON(value, label)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 || bytes.Equal(normalized, []byte("null")) {
		return nil, nil
	}
	if normalized[0] != '{' {
		return nil, fmt.Errorf("bridges: %s must be a JSON object or null", label)
	}
	return normalized, nil
}
