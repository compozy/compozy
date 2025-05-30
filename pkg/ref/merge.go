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
// Replace Mode
// -----------------------------------------------------------------------------

type ReplaceMode struct{}

func (m *ReplaceMode) Merge(refValue, _ any) (any, error) {
	return refValue, nil
}

// -----------------------------------------------------------------------------
// Append Mode
// -----------------------------------------------------------------------------

type AppendMode struct{}

func (m *AppendMode) Merge(refValue, inlineValue any) (any, error) {
	refSlice, refOk := refValue.([]any)
	inlineSlice, inlineOk := inlineValue.([]any)
	if !refOk || !inlineOk {
		return nil, errors.New("append mode only valid on arrays")
	}
	result := make([]any, 0, len(inlineSlice)+len(refSlice))
	result = append(result, inlineSlice...)
	result = append(result, refSlice...)
	return result, nil
}

// -----------------------------------------------------------------------------
// Merge Mode
// -----------------------------------------------------------------------------

type MergeMode struct{}

func (m *MergeMode) Merge(refValue, inlineValue any) (any, error) {
	// Handle nil cases
	if refValue == nil {
		return inlineValue, nil
	}
	if inlineValue == nil {
		return refValue, nil
	}

	// Normalize inputs to map[string]any, []any, or scalars
	refNormalized, inlineNormalized, err := normalizeInputs(refValue, inlineValue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to normalize inputs")
	}

	// Handle slices
	refSlice, refIsSlice := refNormalized.([]any)
	inlineSlice, inlineIsSlice := inlineNormalized.([]any)
	if refIsSlice && inlineIsSlice {
		// Perform union: include all inline elements, then ref elements
		result := make([]any, 0, len(inlineSlice)+len(refSlice))
		for _, inlineItem := range inlineSlice {
			mergedItem, err := m.Merge(nil, inlineItem) // Normalize item
			if err != nil {
				return nil, errors.Wrap(err, "failed to merge inline slice item")
			}
			result = append(result, mergedItem)
		}
		for _, refItem := range refSlice {
			mergedItem, err := m.Merge(refItem, nil) // Normalize item
			if err != nil {
				return nil, errors.Wrap(err, "failed to merge ref slice item")
			}
			result = append(result, mergedItem)
		}
		return result, nil
	}

	// Handle maps
	refMap, refIsMap := refNormalized.(map[string]any)
	inlineMap, inlineIsMap := inlineNormalized.(map[string]any)
	if refIsMap && inlineIsMap {
		// If inline map is empty, just return the ref map
		if len(inlineMap) == 0 {
			return refMap, nil
		}
		// Use Mergo to deep merge maps, with refValue winning conflicts
		result := make(map[string]any, len(inlineMap)+len(refMap))
		// Merge inlineMap first (lower priority)
		if err := mergo.Map(&result, inlineMap, mergo.WithAppendSlice); err != nil {
			return nil, errors.Wrap(err, "failed to merge inline map")
		}
		// Merge refMap with override to prioritize ref values
		if err := mergo.Map(&result, refMap, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
			return nil, errors.Wrap(err, "failed to merge ref map")
		}
		return result, nil
	}

	// For non-map, non-slice types, ref wins
	return refNormalized, nil
}

// normalizeInputs converts inputs to map[string]any, []any, or scalars
func normalizeInputs(refValue, inlineValue any) (any, any, error) {
	refNormalized, err := normalizeValue(refValue)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to normalize ref value")
	}
	inlineNormalized, err := normalizeValue(inlineValue)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to normalize inline value")
	}
	return refNormalized, inlineNormalized, nil
}

// normalizeValue converts structs to map[string]any and handles nested types
func normalizeValue(value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	// Handle maps and slices directly
	switch v := value.(type) {
	case map[string]any:
		// Recursively normalize map values
		result := make(map[string]any, len(v))
		for k, val := range v {
			normalizedVal, err := normalizeValue(val)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to normalize map key %s", k)
			}
			result[k] = normalizedVal
		}
		return result, nil
	case []any:
		// Recursively normalize slice elements
		result := make([]any, len(v))
		for i, val := range v {
			normalizedVal, err := normalizeValue(val)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to normalize slice index %d", i)
			}
			result[i] = normalizedVal
		}
		return result, nil
	}

	// Use reflection for structs
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		result := make(map[string]any)
		t := v.Type()
		for i := range v.NumField() {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			fieldValue := v.Field(i).Interface()
			// Recursively normalize field value
			normalizedField, err := normalizeValue(fieldValue)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to normalize field %s", field.Name)
			}
			// Use JSON/YAML tag if available, otherwise field name
			fieldName := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
				fieldName = strings.Split(jsonTag, ",")[0]
			} else if yamlTag := field.Tag.Get("yaml"); yamlTag != "" && yamlTag != "-" {
				fieldName = strings.Split(yamlTag, ",")[0]
			}
			result[fieldName] = normalizedField
		}
		return result, nil
	}

	// Return non-struct, non-map, non-slice values as-is
	return value, nil
}

// -----------------------------------------------------------------------------
// Apply Merge Mode
// -----------------------------------------------------------------------------

func (r *Ref) ApplyMergeMode(refValue, inlineValue any) (any, error) {
	if r == nil {
		return inlineValue, nil
	}
	var strategy MergeStrategy
	switch r.Mode {
	case ModeReplace:
		strategy = &ReplaceMode{}
	case ModeAppend:
		strategy = &AppendMode{}
	default:
		strategy = &MergeMode{}
	}
	return strategy.Merge(refValue, inlineValue)
}
