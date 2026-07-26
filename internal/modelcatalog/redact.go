package modelcatalog

import redactpkg "github.com/compozy/compozy/internal/redact"

// RedactString removes secret-shaped values from catalog errors.
func RedactString(value string) string {
	return redactpkg.String(value)
}
