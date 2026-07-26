package heartbeat

import (
	"context"

	"errors"
	"fmt"

	"strconv"
	"strings"
	"time"
	"unicode"
)

func normalizeLineEndings(content []byte) []byte {
	replaced := strings.ReplaceAll(string(content), "\r\n", "\n")
	replaced = strings.ReplaceAll(replaced, "\r", "\n")
	return []byte(replaced)
}

func normalizeBody(body string) string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.TrimRightFunc(normalized, unicode.IsSpace)
}

func scalarInt(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case uint64:
		if typed > uint64(^uint(0)>>1) {
			return 0, errors.New("integer overflows int")
		}
		return int(typed), nil
	case float64:
		if typed != float64(int(typed)) {
			return 0, errors.New("number must be an integer")
		}
		return int(typed), nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, fmt.Errorf("parse integer string: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("expected integer, got %T", value)
	}
}

func boolOnly(value any) (bool, error) {
	typed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("expected bool, got %T", value)
	}
	return typed, nil
}

func stringOnly(value any) (string, error) {
	typed, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", value)
	}
	return strings.TrimSpace(typed), nil
}

func stringList(value any, field string) ([]string, error) {
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("HEARTBEAT.md frontmatter field %q must be a list of strings", field)
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for idx, item := range values {
		text, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("HEARTBEAT.md frontmatter field %q[%d] must be a string", field, idx)
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

func parseClock(value string) (time.Time, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse HH:MM clock %q: %w", value, err)
	}
	return parsed, nil
}

func truncateUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	var builder strings.Builder
	for _, r := range value {
		if builder.Len()+len(string(r)) > maxBytes {
			break
		}
		builder.WriteRune(r)
	}
	return strings.TrimSpace(builder.String())
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
