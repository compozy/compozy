package session

import "slices"

func badgeStrings(values []Badge) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, string(value))
	}
	return result
}

func badgeFilterContains(values []Badge, target Badge) bool {
	return slices.Contains(values, target)
}
