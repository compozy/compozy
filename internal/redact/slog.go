package redact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
)

var logContentFields = map[string]struct{}{
	"body":                   {},
	"command":                {},
	"content":                {},
	"description":            {},
	"detail":                 {},
	"error":                  {},
	redactionMessageFieldKey: {},
	"msg":                    {},
	"output":                 {},
	"payload":                {},
	"reason":                 {},
	"result":                 {},
	"stderr":                 {},
	"stdout":                 {},
	"summary":                {},
	"text":                   {},
}

// RedactLogAttrs applies field-aware redaction while preserving correlation,
// numeric, boolean, duration, time, and other structured envelope values.
func (e *Engine) RedactLogAttrs(attrs []slog.Attr) []slog.Attr {
	redacted := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		redacted[i] = e.redactLogAttr(attr)
	}
	return redacted
}

func (e *Engine) redactLogAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	key := normalizeFieldName(attr.Key)
	if attr.Value.Kind() == slog.KindGroup {
		attr.Value = slog.GroupValue(e.RedactLogAttrs(attr.Value.Group())...)
		return attr
	}
	if isProtectedEnvelopeKey(key) {
		return attr
	}
	if IsSensitiveKey(attr.Key) {
		attr.Value = slog.StringValue(Marker)
		return attr
	}
	if _, ok := logContentFields[key]; !ok {
		if attr.Value.Kind() == slog.KindString {
			attr.Value = slog.StringValue(exactRedactString(attr.Value.String()))
		}
		return attr
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(e.RedactString(attr.Value.String()))
	case slog.KindAny:
		value := attr.Value.Any()
		switch typed := value.(type) {
		case error:
			attr.Value = slog.StringValue(e.RedactString(typed.Error()))
		case fmt.Stringer:
			attr.Value = slog.StringValue(e.RedactString(typed.String()))
		default:
			if redacted, changed := e.redactLogStructuredValue(key, value); changed {
				attr.Value = slog.AnyValue(redacted)
			}
		}
	}
	return attr
}

func (e *Engine) redactLogStructuredValue(key string, value any) (any, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return value, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return value, false
	}
	namedFields := map[string]struct{}{normalizeFieldName(key): {}}
	redacted := e.redactJSONValue(decoded, key, namedFields, false)
	if reflect.DeepEqual(decoded, redacted) {
		return value, false
	}
	return redacted, true
}
