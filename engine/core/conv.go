package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// ToStringMap converts supported map forms into map[string]string.
// Note: For map[string]string inputs, this returns a copy to avoid aliasing.
// Unsupported inputs return nil.
func ToStringMap(v any) map[string]string {
	if v == nil {
		return nil
	}
	out := map[string]string{}
	switch m := v.(type) {
	case map[string]string:
		return CloneMap(m)
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
//
// Notes on numeric handling:
//   - int, int64: interpreted as time.Duration units directly.
//   - float64: fractional values are truncated (not rounded) to their integer part
//     before conversion. This is intentional and locked by tests.
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
		if t == float64(int(t)) {
			return int(t), true
		}
		return 0, false
	case string:
		if strings.TrimSpace(t) == "" {
			return 0, false
		}
		if iv, err := strconv.Atoi(t); err == nil {
			return iv, true
		}
		return 0, false
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i), true
		}
		return 0, false
	default:
		return 0, false
	}
}
