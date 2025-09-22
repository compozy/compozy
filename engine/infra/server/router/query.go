package router

import "strings"

func ParseExpandQuery(raw string) map[string]bool {
	result := make(map[string]bool)
	normalized := strings.ReplaceAll(raw, ",", " ")
	fields := strings.Fields(normalized)
	for _, field := range fields {
		result[strings.ToLower(field)] = true
	}
	return result
}
