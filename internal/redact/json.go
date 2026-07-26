package redact

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strings"
)

const (
	redactionMessageFieldKey   = "message"
	redactionSessionIDFieldKey = "session_id"
)

// RedactJSON applies exact protection throughout a JSON document and additive
// heuristics only within the named free-text fields.
func (e *Engine) RedactJSON(raw json.RawMessage, fields []string) json.RawMessage {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return append(json.RawMessage(nil), raw...)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return append(json.RawMessage(nil), raw...)
	}

	namedFields := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		namedFields[normalizeFieldName(field)] = struct{}{}
	}
	redactedValue := e.redactJSONValue(value, "", namedFields, false)
	if reflect.DeepEqual(value, redactedValue) {
		return append(json.RawMessage(nil), raw...)
	}
	redacted, err := json.Marshal(redactedValue)
	if err != nil {
		return append(json.RawMessage(nil), raw...)
	}
	return redacted
}

func (e *Engine) redactJSONValue(value any, key string, namedFields map[string]struct{}, heuristic bool) any {
	normalizedKey := normalizeFieldName(key)
	if isProtectedEnvelopeKey(normalizedKey) {
		switch typed := value.(type) {
		case []any:
			redacted := make([]any, len(typed))
			for i, item := range typed {
				redacted[i] = e.redactJSONValue(item, "", namedFields, heuristic)
			}
			return redacted
		case map[string]any:
			redacted := make(map[string]any, len(typed))
			for childKey, item := range typed {
				redacted[childKey] = e.redactJSONValue(item, childKey, namedFields, heuristic)
			}
			return redacted
		default:
			return value
		}
	}
	if IsSensitiveKey(key) {
		return redactSensitiveJSONValue(value)
	}
	if _, ok := namedFields[normalizedKey]; ok {
		heuristic = true
	}

	switch typed := value.(type) {
	case string:
		if heuristic {
			return e.RedactString(typed)
		}
		return exactRedactString(typed)
	case []any:
		redacted := make([]any, len(typed))
		for i, item := range typed {
			redacted[i] = e.redactJSONValue(item, key, namedFields, heuristic)
		}
		return redacted
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for childKey, item := range typed {
			redacted[childKey] = e.redactJSONValue(item, childKey, namedFields, heuristic)
		}
		return redacted
	default:
		return value
	}
}

func redactSensitiveJSONValue(value any) any {
	switch typed := value.(type) {
	case string:
		if alreadyRedacted(typed) {
			return typed
		}
		return Marker
	case []any:
		redacted := make([]any, len(typed))
		for i, item := range typed {
			redacted[i] = redactSensitiveJSONValue(item)
		}
		return redacted
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			redacted[key] = redactSensitiveJSONValue(item)
		}
		return redacted
	default:
		return value
	}
}

func normalizeFieldName(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func isProtectedEnvelopeKey(key string) bool {
	if strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_hash") ||
		strings.HasSuffix(key, "_digest") || strings.HasSuffix(key, "_fingerprint") {
		return true
	}
	switch key {
	case "claim_token_hash", redactionSessionIDFieldKey, "run_id", "workspace_id", "agent_id",
		"task_id", "goal_id", "loop_id", "fingerprint", "idempotency_key",
		"digest", "schema_digest", "descriptor_digest", "correlation_id", "request_id":
		return true
	default:
		return false
	}
}
