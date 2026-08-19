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
	return e.RedactJSONWithProtection(raw, fields, nil, nil)
}

// RedactJSONWithProtectedStrings applies RedactJSON while preserving scalar
// strings that the caller identifies as public structural handles. The
// predicate is evaluated only after key-based secret protection, so a
// sensitive field name can never opt out of redaction.
func (e *Engine) RedactJSONWithProtectedStrings(
	raw json.RawMessage,
	fields []string,
	protect func(key string, value string) bool,
) json.RawMessage {
	return e.RedactJSONWithProtection(raw, fields, protect, nil)
}

// RedactJSONWithProtection extends RedactJSONWithProtectedStrings with a
// structural-object predicate. The object predicate is consulted only for a
// sensitive field name; accepted objects are traversed normally instead of
// being replaced wholesale.
func (e *Engine) RedactJSONWithProtection(
	raw json.RawMessage,
	fields []string,
	protectString func(key string, value string) bool,
	protectObject func(key string, value any) bool,
) json.RawMessage {
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
	redactedValue := e.redactJSONValue(value, "", namedFields, false, protectString, protectObject)
	// Decoded JSON contains dynamic maps and slices, so structural equality needs
	// reflection. BenchmarkEngineRedactStructuredLogValue tracks this hot path.
	if reflect.DeepEqual(value, redactedValue) {
		return append(json.RawMessage(nil), raw...)
	}
	redacted, err := json.Marshal(redactedValue)
	if err != nil {
		return append(json.RawMessage(nil), raw...)
	}
	return redacted
}

func (e *Engine) redactJSONValue(
	value any,
	key string,
	namedFields map[string]struct{},
	heuristic bool,
	protectString func(key string, value string) bool,
	protectObject func(key string, value any) bool,
) any {
	normalizedKey := normalizeFieldName(key)
	if isProtectedEnvelopeKey(normalizedKey) {
		switch typed := value.(type) {
		case []any:
			redacted := make([]any, len(typed))
			for i, item := range typed {
				redacted[i] = e.redactJSONValue(
					item, "", namedFields, heuristic, protectString, protectObject,
				)
			}
			return redacted
		case map[string]any:
			redacted := make(map[string]any, len(typed))
			for childKey, item := range typed {
				redacted[childKey] = e.redactJSONValue(
					item, childKey, namedFields, heuristic, protectString, protectObject,
				)
			}
			return redacted
		default:
			return value
		}
	}
	if IsSensitiveKey(key) && (protectObject == nil || !protectObject(key, value)) {
		return redactSensitiveJSONValue(value)
	}
	if _, ok := namedFields[normalizedKey]; ok {
		heuristic = true
	}

	switch typed := value.(type) {
	case string:
		if protectString != nil && protectString(key, typed) {
			return typed
		}
		if heuristic {
			return e.RedactString(typed)
		}
		return exactRedactString(typed)
	case []any:
		redacted := make([]any, len(typed))
		for i, item := range typed {
			redacted[i] = e.redactJSONValue(
				item, key, namedFields, heuristic, protectString, protectObject,
			)
		}
		return redacted
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for childKey, item := range typed {
			redacted[childKey] = e.redactJSONValue(
				item, childKey, namedFields, heuristic, protectString, protectObject,
			)
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
		strings.HasSuffix(key, "_digest") || strings.HasSuffix(key, "_fingerprint") ||
		strings.HasSuffix(key, "_cursor") {
		return true
	}
	switch key {
	case "claim_token_hash", redactionSessionIDFieldKey, "run_id", "workspace_id", "agent_id",
		"task_id", "goal_id", "loop_id", "fingerprint", "idempotency_key",
		"digest", "schema_digest", "descriptor_digest", "correlation_id", "request_id", "cursor":
		return true
	default:
		return false
	}
}
