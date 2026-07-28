package cli

import (
	"fmt"

	"sort"
)

func flattenConfigValue(entries *[]configEntry, path string, value any, redacted bool) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			if path != "" {
				*entries = append(*entries, configEntry{Path: path, Value: map[string]any{}, Redacted: redacted})
			}
			return
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			flattenConfigValue(
				entries,
				nextPath,
				typed[key],
				redacted || key == configEnvKey || key == configSecretEnvKey,
			)
		}
	case []any:
		if len(typed) == 0 {
			if path != "" {
				*entries = append(*entries, configEntry{Path: path, Value: []any{}, Redacted: redacted})
			}
			return
		}
		for i, item := range typed {
			flattenConfigValue(entries, fmt.Sprintf("%s[%d]", path, i), item, redacted)
		}
	default:
		if path != "" {
			*entries = append(*entries, configEntry{Path: path, Value: typed, Redacted: redacted})
		}
	}
}
