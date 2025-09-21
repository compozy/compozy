package router

import "strings"

func ParseFieldsQuery(raw string) map[string]bool {
	result := make(map[string]bool)
	parts := strings.Split(raw, ",")
	for i := range parts {
		trimmed := strings.TrimSpace(parts[i])
		if trimmed != "" {
			result[trimmed] = true
		}
	}
	return result
}

func ParseExpandQuery(raw string) map[string]bool {
	result := make(map[string]bool)
	parts := strings.Split(raw, ",")
	for i := range parts {
		trimmed := strings.TrimSpace(parts[i])
		if trimmed != "" {
			result[strings.ToLower(trimmed)] = true
		}
	}
	return result
}

func FilterMapFields(data map[string]any, fields map[string]bool) map[string]any {
	if len(fields) == 0 {
		return data
	}
	out := make(map[string]any, len(fields))
	for key := range fields {
		if val, ok := data[key]; ok {
			out[key] = val
		}
	}
	return out
}
