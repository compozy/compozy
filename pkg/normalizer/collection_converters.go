package normalizer

import (
	"reflect"
	"sort"
)

// TypeConverter handles conversion of various types to slices for collection processing
type TypeConverter struct{}

// NewTypeConverter creates a new type converter
func NewTypeConverter() *TypeConverter {
	return &TypeConverter{}
}

// ConvertToSlice converts various types to a slice of interfaces
func (tc *TypeConverter) ConvertToSlice(value any) []any {
	if value == nil {
		return []any{}
	}

	switch v := value.(type) {
	case []any:
		return v
	case []string:
		return tc.convertStringSlice(v)
	case []int:
		return tc.convertIntSlice(v)
	case []float64:
		return tc.convertFloatSlice(v)
	case map[string]any:
		return tc.convertMapToSlice(v)
	case string, int, int32, int64, float32, float64, bool:
		return []any{v}
	default:
		return tc.convertReflectionSlice(value)
	}
}

// convertStringSlice converts []string to []any
func (tc *TypeConverter) convertStringSlice(v []string) []any {
	result := make([]any, len(v))
	for i, item := range v {
		result[i] = item
	}
	return result
}

// convertIntSlice converts []int to []any
func (tc *TypeConverter) convertIntSlice(v []int) []any {
	result := make([]any, len(v))
	for i, item := range v {
		result[i] = item
	}
	return result
}

// convertFloatSlice converts []float64 to []any
func (tc *TypeConverter) convertFloatSlice(v []float64) []any {
	result := make([]any, len(v))
	for i, item := range v {
		result[i] = item
	}
	return result
}

// convertMapToSlice converts map to slice of key-value pairs
func (tc *TypeConverter) convertMapToSlice(v map[string]any) []any {
	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(v))
	for key := range v {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]any, 0, len(v))
	for _, key := range keys {
		result = append(result, map[string]any{
			"key":   key,
			"value": v[key],
		})
	}
	return result
}

// convertReflectionSlice handles any slice/array type using reflection
func (tc *TypeConverter) convertReflectionSlice(value any) []any {
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		result := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			result[i] = rv.Index(i).Interface()
		}
		return result
	}
	// If it's not a slice/array/map, treat as single item
	return []any{value}
}
