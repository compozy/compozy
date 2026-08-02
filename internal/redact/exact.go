package redact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errExactJSONKeyCollision = errors.New("redact: JSON keys collide after exact redaction")

// ExactString removes canonical and registered secrets without additive heuristics.
func ExactString(value string) string {
	return exactRedactString(value)
}

// ExactJSON removes canonical and registered secrets from JSON keys and string values.
func ExactJSON(raw json.RawMessage) (json.RawMessage, error) {
	return exactJSON(raw, exactRedactString)
}

func exactJSON(raw json.RawMessage, redactString func(string) string) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return append(json.RawMessage(nil), raw...), nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("redact: decode exact JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("redact: exact JSON contains trailing data")
		}
		return nil, fmt.Errorf("redact: decode exact JSON trailing data: %w", err)
	}

	redacted, err := exactJSONValue(value, redactString)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(redacted)
	if err != nil {
		return nil, fmt.Errorf("redact: encode exact JSON: %w", err)
	}
	return json.RawMessage(encoded), nil
}

func exactJSONValue(value any, redactString func(string) string) (any, error) {
	switch typed := value.(type) {
	case string:
		return redactString(typed), nil
	case []any:
		redacted := make([]any, len(typed))
		for index := range typed {
			item, err := exactJSONValue(typed[index], redactString)
			if err != nil {
				return nil, err
			}
			redacted[index] = item
		}
		return redacted, nil
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, rawItem := range typed {
			redactedKey := redactString(key)
			if _, exists := redacted[redactedKey]; exists {
				return nil, errExactJSONKeyCollision
			}
			item, err := exactJSONValue(rawItem, redactString)
			if err != nil {
				return nil, err
			}
			redacted[redactedKey] = item
		}
		return redacted, nil
	default:
		return value, nil
	}
}
