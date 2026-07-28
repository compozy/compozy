package store

import "strings"

const defaultSessionType = "user"

// NormalizeSessionType applies the default session type when empty.
func NormalizeSessionType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultSessionType
	}
	return value
}
