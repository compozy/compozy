package router

import (
	"fmt"
	"strings"
)

func ParseStrongETag(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if strings.Contains(trimmed, ",") {
		parts := strings.Split(trimmed, ",")
		if len(parts) == 0 {
			return "", fmt.Errorf("invalid etag header")
		}
		trimmed = strings.TrimSpace(parts[0])
	}
	if trimmed == "" || trimmed == "*" {
		return "", fmt.Errorf("invalid etag header")
	}
	if strings.HasPrefix(trimmed, "W/") || strings.HasPrefix(trimmed, "w/") {
		return "", fmt.Errorf("weak etag not allowed")
	}
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") && len(trimmed) >= 2 {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	if trimmed == "" {
		return "", fmt.Errorf("invalid etag header")
	}
	if strings.HasPrefix(trimmed, "W/") || strings.HasPrefix(trimmed, "w/") {
		return "", fmt.Errorf("weak etag not allowed")
	}
	return trimmed, nil
}
