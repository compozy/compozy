package router

import "strings"

func ParseFieldsQuery(raw string) map[string]bool {
	result := make(map[string]bool)
	normalized := strings.ReplaceAll(raw, ",", " ")
	fields := strings.Fields(normalized)
	for _, field := range fields {
		result[field] = true
	}
	return result
}

func ParseExpandQuery(raw string) map[string]bool {
	result := make(map[string]bool)
	normalized := strings.ReplaceAll(raw, ",", " ")
	fields := strings.Fields(normalized)
	for _, field := range fields {
		result[strings.ToLower(field)] = true
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
