package ref

import (
	"reflect"
	"strings"

	"dario.cat/mergo"
	"github.com/pkg/errors"
)

// MergeStrategy defines a strategy for merging reference and inline values.
type MergeStrategy interface {
	Merge(refValue, inlineValue any) (any, error)
}

// -----------------------------------------------------------------------------
// Strategy Factory
// -----------------------------------------------------------------------------

// GetMergeStrategy returns the appropriate merge strategy for the given mode.
func GetMergeStrategy(mode Mode) (MergeStrategy, error) {
	switch mode {
	case ModeReplace:
		return &ReplaceStrategy{}, nil
	case ModeAppend:
		return &AppendStrategy{}, nil
	case ModeMerge:
		return &DeepMergeStrategy{}, nil
	default:
		return nil, errors.Errorf("unknown merge mode: %s", mode)
	}
}

// -----------------------------------------------------------------------------
// Replace Strategy
// -----------------------------------------------------------------------------

type ReplaceStrategy struct{}

func (s *ReplaceStrategy) Merge(refValue, _ any) (any, error) {
	return refValue, nil
}

// -----------------------------------------------------------------------------
// Append Strategy
// -----------------------------------------------------------------------------

type AppendStrategy struct{}

func (s *AppendStrategy) Merge(refValue, inlineValue any) (any, error) {
	refSlice, refOk := toSlice(refValue)
	inlineSlice, inlineOk := toSlice(inlineValue)

	if !refOk || !inlineOk {
		return nil, errors.New("append mode requires both values to be arrays")
	}

	// Create result with inline values first, then ref values
	result := make([]any, 0, len(inlineSlice)+len(refSlice))
	result = append(result, inlineSlice...)
	result = append(result, refSlice...)
	return result, nil
}

// -----------------------------------------------------------------------------
// Deep Merge Strategy
// -----------------------------------------------------------------------------

type DeepMergeStrategy struct{}

func (s *DeepMergeStrategy) Merge(refValue, inlineValue any) (any, error) {
	// Handle nil cases
	if refValue == nil {
		return inlineValue, nil
	}
	if inlineValue == nil {
		return refValue, nil
	}

	// Try to merge as maps
	refMap, refIsMap := toMap(refValue)
	inlineMap, inlineIsMap := toMap(inlineValue)

	if refIsMap && inlineIsMap {
		return s.mergeMaps(refMap, inlineMap)
	}

	// Try to merge as slices
	refSlice, refIsSlice := toSlice(refValue)
	inlineSlice, inlineIsSlice := toSlice(inlineValue)

	if refIsSlice && inlineIsSlice {
		return s.mergeSlices(refSlice, inlineSlice), nil
	}

	// For different types or scalar values, ref wins (takes precedence)
	return refValue, nil
}

func (s *DeepMergeStrategy) mergeMaps(refMap, inlineMap map[string]any) (any, error) {
	// If inline map is empty, just return ref map
	if len(inlineMap) == 0 {
		return refMap, nil
	}

	// Fast path for flat maps (no nested maps or slices)
	if isFlat(refMap) && isFlat(inlineMap) {
		result := make(map[string]any, len(inlineMap)+len(refMap))
		// First copy inline values (base)
		for k, v := range inlineMap {
			result[k] = v
		}
		// Then apply ref values (override)
		for k, v := range refMap {
			result[k] = v
		}
		return result, nil
	}

	// Create result map with proper capacity
	result := make(map[string]any, len(inlineMap)+len(refMap))

	// First copy inline values (base)
	for k, v := range inlineMap {
		result[k] = v
	}

	// Then apply ref values (override), recursively merging nested structures
	for k, refVal := range refMap {
		if inlineVal, exists := inlineMap[k]; exists {
			// Recursively merge nested values
			merged, err := s.Merge(refVal, inlineVal)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to merge key '%s'", k)
			}
			result[k] = merged
		} else {
			result[k] = refVal
		}
	}

	return result, nil
}

func (s *DeepMergeStrategy) mergeSlices(refSlice, inlineSlice []any) []any {
	// For slices, perform union: inline elements first, then ref elements
	result := make([]any, 0, len(inlineSlice)+len(refSlice))
	result = append(result, inlineSlice...)
	result = append(result, refSlice...)
	return result
}

// isFlat checks if a map contains only primitive values (no nested maps or slices).
func isFlat(m map[string]any) bool {
	for _, v := range m {
		switch v.(type) {
		case map[string]any, []any:
			return false
		case map[any]any, []map[string]any, []map[any]any:
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// toMap attempts to convert a value to map[string]any.
func toMap(value any) (map[string]any, bool) {
	// Direct type assertion
	if m, ok := value.(map[string]any); ok {
		return m, true
	}

	// Handle nil
	if value == nil {
		return nil, false
	}

	// Use reflection for struct conversion
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		return structToMap(v), true
	}

	return nil, false
}

// toSlice attempts to convert a value to []any.
func toSlice(value any) ([]any, bool) {
	// Direct type assertion
	if s, ok := value.([]any); ok {
		return s, true
	}

	// Handle nil
	if value == nil {
		return nil, false
	}

	// Use reflection for other slice types
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Slice {
		result := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = v.Index(i).Interface()
		}
		return result, true
	}

	return nil, false
}

// structToMap converts a struct to map[string]any using reflection.
func structToMap(v reflect.Value) map[string]any {
	result := make(map[string]any)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldValue := v.Field(i).Interface()
		fieldName := getFieldName(&field)

		// Recursively convert nested structs
		if nestedMap, ok := toMap(fieldValue); ok {
			result[fieldName] = nestedMap
		} else if nestedSlice, ok := toSlice(fieldValue); ok {
			result[fieldName] = nestedSlice
		} else {
			result[fieldName] = fieldValue
		}
	}

	return result
}

// getFieldName extracts the field name from tags.
func getFieldName(field *reflect.StructField) string {
	// Check JSON tag first
	if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
		if idx := strings.Index(jsonTag, ","); idx > 0 {
			return jsonTag[:idx]
		}
		return jsonTag
	}

	// Check YAML tag
	if yamlTag := field.Tag.Get("yaml"); yamlTag != "" && yamlTag != "-" {
		if idx := strings.Index(yamlTag, ","); idx > 0 {
			return yamlTag[:idx]
		}
		return yamlTag
	}

	return field.Name
}

// -----------------------------------------------------------------------------
// Apply Merge Mode
// -----------------------------------------------------------------------------

// ApplyMergeMode applies the appropriate merge strategy based on the mode.
func (r *Ref) ApplyMergeMode(refValue, inlineValue any) (any, error) {
	if r == nil {
		return inlineValue, nil
	}

	strategy, err := GetMergeStrategy(r.Mode)
	if err != nil {
		return nil, err
	}

	return strategy.Merge(refValue, inlineValue)
}

// -----------------------------------------------------------------------------
// Deprecated: Legacy merge implementation using mergo
// -----------------------------------------------------------------------------

// LegacyMergeMode uses the mergo library for backward compatibility.
type LegacyMergeMode struct{}

func (m *LegacyMergeMode) Merge(refValue, inlineValue any) (any, error) {
	refMap, refOk := refValue.(map[string]any)
	inlineMap, inlineOk := inlineValue.(map[string]any)

	if !refOk || !inlineOk {
		return refValue, nil
	}

	result := make(map[string]any)
	if err := mergo.Map(&result, inlineMap, mergo.WithAppendSlice); err != nil {
		return nil, errors.Wrap(err, "failed to merge inline map")
	}
	if err := mergo.Map(&result, refMap, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return nil, errors.Wrap(err, "failed to merge ref map")
	}

	return result, nil
}
