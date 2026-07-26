package diagnostics

import (
	"strings"

	redactpkg "github.com/compozy/agh/internal/redact"
)

const redactedValue = redactpkg.Marker
const truncationSuffix = "...[truncated]"

// RegisterDynamicSecret registers one runtime-resolved secret for diagnostic redaction.
func RegisterDynamicSecret(value string) func() {
	return redactpkg.RegisterDynamicSecret(value)
}

// Redact removes common credential shapes from diagnostic text before the text
// is persisted or exposed to operators.
func Redact(text string) string {
	return redactpkg.String(text)
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
