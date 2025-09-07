package core

import (
	"strconv"
	"strings"
	"time"
)

// ToStringMap converts supported map forms into map[string]string. Unsupported inputs return nil.
func ToStringMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	out := map[string]string{}
	switch m := v.(type) {
	case map[string]string:
		return m
	case map[string]any:
		for k, vv := range m {
			if s, ok := vv.(string); ok {
				out[k] = s
			}
		}
		return out
	default:
		return nil
	}
}

// ParseAnyDuration parses a duration from common forms. Returns false when unsupported.
func ParseAnyDuration(v any) (time.Duration, bool) {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return 0, false
		}
		if d, err := ParseHumanDuration(t); err == nil {
			return d, true
		}
		return 0, false
	case int:
		return time.Duration(t), true
	case int64:
		return time.Duration(t), true
	case float64:
		return time.Duration(int64(t)), true
	default:
		return 0, false
	}
}

// ParseAnyInt parses an integer from common forms. Returns false when unsupported.
func ParseAnyInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case string:
		if strings.TrimSpace(t) == "" {
			return 0, false
		}
		if iv, err := strconv.Atoi(t); err == nil {
			return iv, true
		}
		return 0, false
	default:
		return 0, false
	}
}
