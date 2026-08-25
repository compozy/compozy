package contracts

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderUntrusted labels, sanitizes, and bounds an untrusted value for prompt projection.
func RenderUntrusted(label string, value any, maxBytes int) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		encoded = fmt.Append(nil, value)
	}
	clean, _, reject := SanitizeText(string(encoded))
	if reject {
		clean = "[REDACTED]"
	}
	prefix := fmt.Sprintf("[untrusted %s]\n", strings.TrimSpace(label))
	suffix := "\n[/untrusted]"
	if maxBytes <= len(prefix)+len(suffix) {
		return boundedUTF8([]byte(prefix+suffix), maxBytes)
	}
	content := boundedUTF8([]byte(clean), maxBytes-len(prefix)-len(suffix))
	return prefix + content + suffix
}
