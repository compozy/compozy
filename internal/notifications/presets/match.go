package presets

import "strings"

func MatchesAny(patterns []string, eventType string) bool {
	trimmed := strings.TrimSpace(eventType)
	if trimmed == "" {
		return false
	}
	for _, pattern := range patterns {
		if MatchesEvent(pattern, trimmed) {
			return true
		}
	}
	return false
}

func MatchesEvent(pattern string, eventType string) bool {
	normalizedPattern := strings.TrimSpace(pattern)
	normalizedEvent := strings.TrimSpace(eventType)
	if normalizedPattern == "" || normalizedEvent == "" {
		return false
	}
	if !strings.Contains(normalizedPattern, "*") {
		return normalizedPattern == normalizedEvent
	}
	if strings.Count(normalizedPattern, "*") != 1 || !strings.HasSuffix(normalizedPattern, "*") {
		return false
	}
	prefix := strings.TrimSuffix(normalizedPattern, "*")
	return prefix != "" && strings.HasPrefix(normalizedEvent, prefix)
}
