package update

import "strings"

func firstPath(primary string, fallback string) string {
	if value := strings.TrimSpace(primary); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
