package diagnostics

import (
	"encoding/json"
	"strings"

	redactpkg "github.com/compozy/compozy/internal/redact"
)

const redactedValue = redactpkg.Marker
const truncationSuffix = "...[truncated]"

// RegisterDynamicSecret registers one runtime-resolved secret for diagnostic redaction.
func RegisterDynamicSecret(value string) func() {
	return redactpkg.RegisterDynamicSecret(value)
}

// RegisterRequiredSecret registers one caller-classified secret without a length heuristic.
func RegisterRequiredSecret(value string) func() {
	return redactpkg.RegisterRequiredSecret(value)
}

// Redact removes common credential shapes from diagnostic text before the text
// is persisted or exposed to operators.
func Redact(text string) string {
	return redactpkg.String(text)
}

// RedactExact removes canonical and registered secrets without additive heuristics.
func RedactExact(text string) string {
	return redactpkg.ExactString(text)
}

// RedactExactBinary removes literal and reversibly encoded registered secrets from text.
func RedactExactBinary(text string) string {
	return redactpkg.ExactBinaryString(text)
}

// RedactExactBytes removes registered secrets from opaque bytes before they are encoded.
func RedactExactBytes(value []byte) []byte {
	return redactpkg.ExactBytes(value)
}

// RedactExactJSON removes canonical and registered secrets from structured JSON.
func RedactExactJSON(raw json.RawMessage) (json.RawMessage, error) {
	return redactpkg.ExactJSON(raw)
}

// RedactExactBinaryJSON removes literal and reversibly encoded registered secrets from structured JSON.
func RedactExactBinaryJSON(raw json.RawMessage) (json.RawMessage, error) {
	return redactpkg.ExactBinaryJSON(raw)
}

// RedactAndBound redacts diagnostic text and caps it to a deterministic byte
// budget. Callers should use this before storing crash evidence.
func RedactAndBound(text string, maxBytes int) string {
	redacted := strings.TrimSpace(Redact(text))
	if maxBytes <= 0 {
		return ""
	}
	if len(redacted) <= maxBytes {
		return redacted
	}
	if maxBytes <= len(truncationSuffix) {
		return truncateUTF8WithinBytes(redacted, maxBytes)
	}
	return truncateUTF8WithinBytes(redacted, maxBytes-len(truncationSuffix)) + truncationSuffix
}

func truncateUTF8WithinBytes(text string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(text) <= maxBytes {
		return text
	}
	boundary := 0
	for idx := range text {
		if idx > maxBytes {
			break
		}
		boundary = idx
	}
	return text[:boundary]
}
