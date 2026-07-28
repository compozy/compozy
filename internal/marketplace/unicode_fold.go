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
