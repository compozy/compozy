package calls

import (
	"strings"

	"github.com/compozy/compozy/internal/contracts"
)

func sanitizeDiagnostic(value, fallback string) string {
	clean, _, reject := contracts.SanitizeText(strings.TrimSpace(value))
	if reject || strings.TrimSpace(clean) == "" {
		return fallback
	}
	return clean
}
