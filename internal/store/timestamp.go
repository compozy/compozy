package store

import (
	"fmt"
	"strings"
	"time"
)

const timestampLayout = "2006-01-02T15:04:05.000000000Z"

func normalizeTime(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return value.UTC()
}

// FormatTimestamp renders a timestamp in the canonical SQLite text layout.
func FormatTimestamp(value time.Time) string {
	return normalizeTime(value).Format(timestampLayout)
}

// ParseTimestamp parses the canonical SQLite text timestamp.
func ParseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse timestamp %q: %w", value, err)
	}
	return parsed.UTC(), nil
}

// FormatNullableTimestamp renders zero timestamps as empty strings for optional columns.
func FormatNullableTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return FormatTimestamp(value)
}

// ParseNullableTimestamp parses optional canonical timestamps.
func ParseNullableTimestamp(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := ParseTimestamp(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
