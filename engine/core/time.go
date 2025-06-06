package core

import (
	"strings"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"
)

// -----------------------------------------------------------------------------
// Human-readable Duration Parser
// -----------------------------------------------------------------------------

// ParseHumanDuration parses human-readable duration strings like "3 days", "1 hour", "30 minutes"
// First tries standard Go duration format (e.g., "30m", "1h30m"), then falls back to str2duration
// for more complex formats like "1 day 2 hours 3 minutes"
func ParseHumanDuration(s string) (time.Duration, error) {
	// First try standard Go duration parsing
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Convert common human-readable formats to Go format
	converted := convertHumanToGoFormat(s)
	if converted != s {
		if d, err := time.ParseDuration(converted); err == nil {
			return d, nil
		}
	}

	// Fall back to str2duration for complex formats
	return str2duration.ParseDuration(s)
}

// convertHumanToGoFormat converts simple human-readable formats to Go duration format
func convertHumanToGoFormat(s string) string {
	// Handle basic patterns like "N seconds", "N minutes", "N hours"
	switch {
	case strings.HasSuffix(s, " second"):
		return strings.Replace(s, " second", "s", 1)
	case strings.HasSuffix(s, " seconds"):
		return strings.Replace(s, " seconds", "s", 1)
	case strings.HasSuffix(s, " minute"):
		return strings.Replace(s, " minute", "m", 1)
	case strings.HasSuffix(s, " minutes"):
		return strings.Replace(s, " minutes", "m", 1)
	case strings.HasSuffix(s, " hour"):
		return strings.Replace(s, " hour", "h", 1)
	case strings.HasSuffix(s, " hours"):
		return strings.Replace(s, " hours", "h", 1)
	default:
		return s
	}
}
