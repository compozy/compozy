package loop

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// RetiredRuntimeKeyPath returns the first deterministic path to a retired runtime key.
func RetiredRuntimeKeyPath(raw []byte) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return walkRetiredRuntimeKey(value, nil, "", false)
}

func walkRetiredRuntimeKey(value any, path []string, parentKey string, criterion bool) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			nextPath := appendRuntimeKeyPath(path, key)
			if key == "model_defaults" ||
				(key == runtimeFieldModel && (parentKey == "params" || criterion)) {
				return strings.Join(nextPath, "."), true
			}
			nextCriterion := key == "criteria" || key == "verification"
			if foundPath, found := walkRetiredRuntimeKey(typed[key], nextPath, key, nextCriterion); found {
				return foundPath, true
			}
		}
	case []any:
		for index, item := range typed {
			nextPath := appendRuntimeKeyPath(path, fmt.Sprintf("[%d]", index))
			if foundPath, found := walkRetiredRuntimeKey(item, nextPath, parentKey, criterion); found {
				return foundPath, true
			}
		}
	}
	return "", false
}

func appendRuntimeKeyPath(path []string, segment string) []string {
	cloned := make([]string, len(path), len(path)+1)
	copy(cloned, path)
	if strings.HasPrefix(segment, "[") && len(cloned) > 0 {
		cloned[len(cloned)-1] += segment
		return cloned
	}
	return append(cloned, segment)
}
