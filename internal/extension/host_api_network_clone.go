package extensionpkg

import (
	"encoding/json"
	"strings"
	"time"
)

func hostAPITimeValuePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}

func hostAPICloneTimePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	return hostAPITimeValuePtr(*value)
}

func hostAPICloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func hostAPICloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := strings.TrimSpace(*value)
	return &copyValue
}

func hostAPIPtrString(value string) *string {
	copyValue := strings.TrimSpace(value)
	return &copyValue
}

func hostAPICloneTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			cloned = append(cloned, trimmed)
		}
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func hostAPICloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func hostAPICloneRawMap[T ~map[string]json.RawMessage](source T) map[string]json.RawMessage {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		cloned[key] = hostAPICloneRawMessage(value)
	}
	return cloned
}
