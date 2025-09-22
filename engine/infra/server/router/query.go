package router

import (
	"strings"
	"unicode"
)

// -----------------------------------------------------------------------------
// Query parsing helpers
// -----------------------------------------------------------------------------

// ParseExpandQuery parses a comma- or whitespace-separated "expand" query value
// into a lowercase set of unique keys.
func ParseExpandQuery(raw string) map[string]bool {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	result := make(map[string]bool, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		result[strings.ToLower(field)] = true
	}
	return result
}

func ParseExpandQueries(values []string) map[string]bool {
	result := make(map[string]bool)
	for _, value := range values {
		for key := range ParseExpandQuery(value) {
			result[key] = true
		}
	}
	return result
}
