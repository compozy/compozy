package marketplace

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var marketplaceCaseFolder = cases.Fold()

func foldMarketplaceText(value string) string {
	return norm.NFC.String(marketplaceCaseFolder.String(strings.TrimSpace(value)))
}

// NormalizeQuery applies the shared catalog query limit in Unicode characters.
func NormalizeQuery(query string) string {
	chars := []rune(strings.TrimSpace(query))
	if len(chars) > 200 {
		chars = chars[:200]
	}
	return string(chars)
}
