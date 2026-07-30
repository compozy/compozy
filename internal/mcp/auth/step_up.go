package auth

import (
	"strings"
)

func unionScopes(current []string, additional []string) []string {
	seen := make(map[string]struct{}, len(current)+len(additional))
	result := make([]string, 0, len(current)+len(additional))
	for _, scope := range append(append([]string(nil), current...), additional...) {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
