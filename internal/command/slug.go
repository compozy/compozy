package command

import (
	"strings"
	"unicode"
)

// Slug returns the canonical ASCII identifier used inside slash tokens.
func Slug(value string) string {
	var builder strings.Builder
	wroteSeparator := false
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		isAllowed := character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-'
		if isAllowed {
			builder.WriteRune(character)
			wroteSeparator = character == '-'
			continue
		}
		if unicode.IsSpace(character) || character > unicode.MaxASCII || !isAllowed {
			if builder.Len() > 0 && !wroteSeparator {
				builder.WriteByte('-')
				wroteSeparator = true
			}
		}
	}
	return strings.Trim(builder.String(), "-._")
}
