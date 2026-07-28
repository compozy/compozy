package automation

import "maps"

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = cloneAnyValue(value)
	}
	return cloned
}

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case map[string]string:
		return cloneStringMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for idx, item := range typed {
			cloned[idx] = cloneAnyValue(item)
		}
		return cloned
	case []string:
		return append([]string(nil), typed...)
	case []byte:
		return append([]byte(nil), typed...)
	case []map[string]any:
		cloned := make([]map[string]any, len(typed))
		for idx, item := range typed {
			cloned[idx] = cloneAnyMap(item)
		}
		return cloned
	case []map[string]string:
		cloned := make([]map[string]string, len(typed))
		for idx, item := range typed {
			cloned[idx] = cloneStringMap(item)
		}
		return cloned
	default:
		return value
	}
}
