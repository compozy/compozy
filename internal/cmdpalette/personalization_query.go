package cmdpalette

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeQuery returns the only query representation eligible for durable usage recording.
func NormalizeQuery(query string) string {
	normalized := strings.ToLower(norm.NFKD.String(strings.TrimSpace(query)))
	withoutMarks := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, normalized)
	return strings.Join(strings.Fields(withoutMarks), " ")
}
