package ref

import (
	"reflect"

	"dario.cat/mergo"
	"github.com/pkg/errors"
)

// ApplyMergeMode applies the merge strategy based on the Ref's mode.
// It merges refValue into inlineValue based on the specified r.Mode.
//
// Behavior:
// - ModeReplace: refValue is returned, inlineValue is ignored.
// - ModeAppend:
//   - If both refValue and inlineValue are slices, they are concatenated (inlineValue first, then refValue).
//   - If one is a slice and the other is nil, the slice is returned.
//   - Otherwise, an error is returned.
// - ModeMerge:
//   - If both refValue and inlineValue are maps (map[string]any), they are merged.
//     The inlineMap serves as the base, and refMap is merged into it.
//     `mergo.WithOverride` ensures that values from refMap replace values in inlineMap for common keys.
//     `mergo.WithAppendSlice` ensures that if both maps have slices for the same key, the slices are appended.
//   - If either value is nil, the non-nil value is returned.
//   - If types are different and not both maps (e.g., map and a slice, or scalar types),
//     refValue takes precedence and is returned. This aligns with the previous behavior
//     where the referenced content was considered dominant in case of type mismatch for merging.
// - Default: An error is returned for any unknown merge mode.
func (r *Ref) ApplyMergeMode(refValue, inlineValue any) (any, error) {
	if r == nil {
		// This case should ideally not be reached if called on a valid Ref instance.
		// If r is nil, we might default to returning inlineValue or error.
		// The original code returned inlineValue, errors.New("ApplyMergeMode called on nil Ref") seems safer.
		return nil, errors.New("ApplyMergeMode called on nil Ref instance")
	}

	switch r.Mode {
	case ModeReplace:
		return refValue, nil
	case ModeAppend:
		return appendValues(refValue, inlineValue)
	case ModeMerge:
		return mergeValues(refValue, inlineValue)
	default:
		// Fallback to ModeMerge if mode is empty or not set, common in tests or older configs
		if r.Mode == "" {
			// Defaulting to Merge as it's often the implicit expectation
			return mergeValues(refValue, inlineValue)
		}
		return nil, errors.Errorf("unknown merge mode: '%s'", r.Mode)
	}
}

func mergeValues(refValue, inlineValue any) (any, error) {
	// If either is nil, return the other value directly.
	if refValue == nil {
		return inlineValue, nil
	}
	if inlineValue == nil {
		return refValue, nil
	}

	// Attempt to assert to map[string]any for mergo.
	// We make copies to avoid modifying the original input maps/slices.
	refMap, okRef := refValue.(map[string]any)
	inlineMap, okInline := inlineValue.(map[string]any)

	if okRef && okInline {
		// Create a deep copy of inlineMap to serve as the destination (dst).
		// This prevents modification of the original inlineValue.
		dst := make(map[string]any)
		// mergo.Map(&dst, inlineMap, mergo.WithOverride) // WithOverride to ensure it's a full copy
		// A simpler way to copy for this case:
		for k, v := range inlineMap {
			dst[k] = v // This is a shallow copy of the map's top level.
			              // For deep copy of nested structures, a proper deep copy func would be needed
									  // or rely on mergo to handle it if we merge into an empty map first.
									  // Let's use mergo for a clean deep copy into dst.
		}
		// Re-initialize dst for a clean deep copy by mergo
		dst = make(map[string]any)
		if err := mergo.Map(&dst, inlineMap, mergo.WithOverride); err != nil {
			return nil, errors.Wrap(err, "failed to deep copy inline map for merge")
		}

		// Merge refMap into dst.
		// - mergo.WithOverride: Values from refMap will overwrite values in dst for the same keys.
		// - mergo.WithAppendSlice: Slices for the same key in both maps will be appended.
		if err := mergo.Map(&dst, refMap, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
			return nil, errors.Wrap(err, "failed to merge refValue into inlineValue")
		}
		return dst, nil
	}

	// If not both are maps (e.g., one is a map, the other a slice, or scalar types),
	// the refValue takes precedence. This is consistent with the previous behavior
	// where the external reference content is considered dominant if a structural merge isn't possible.
	return refValue, nil
}

func appendValues(refValue, inlineValue any) (any, error) {
	// Handle cases where one or both values are nil.
	if refValue == nil && inlineValue == nil {
		return []any{}, nil // Or return nil, depending on desired behavior for two nils. Empty slice is safer.
	}

	refSlice, refIsSlice := toSliceE(refValue)
	inlineSlice, inlineIsSlice := toSliceE(inlineValue)

	if refIsSlice && inlineIsSlice {
		// Both are slices, concatenate them: inline elements first, then ref elements.
		return append(inlineSlice, refSlice...), nil
	}

	// If one is a slice and the other is nil, return the slice.
	if refIsSlice && inlineValue == nil {
		return refSlice, nil // Or a copy: append([]any(nil), refSlice...)
	}
	if inlineIsSlice && refValue == nil {
		return inlineSlice, nil // Or a copy: append([]any(nil), inlineSlice...)
	}

	// If we reach here, types are incompatible for append (e.g., map and slice, or scalar).
	// The ModeAppend is primarily intended for slices.
	// Error if types are not amenable to appending.
	// It's important to clarify behavior: if one is a slice and the other is not (and not nil), is it an error?
	// The original AppendStrategy strictly required both to be arrays/slices.
	// The prompt's example errors out if not both slices.
	errMsg := "append mode requires both values to be slices"
	if refValue != nil && !refIsSlice {
		errMsg = errMsg + errors.Errorf("; ref value is %T", refValue).Error()
	}
	if inlineValue != nil && !inlineIsSlice {
		errMsg = errMsg + errors.Errorf("; inline value is %T", inlineValue).Error()
	}
	return nil, errors.New(errMsg)
}

// toSliceE converts a value to []any if it's a slice.
// It returns the converted slice and a boolean indicating success.
func toSliceE(value any) ([]any, bool) {
	if value == nil {
		return nil, false // Nil is not considered a slice here. Or true, with nil slice.
		                 // For append, if one is nil, it's better to treat it as an empty slice implicitly.
										 // However, the appendValues logic handles nil explicitly first.
	}
	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Slice {
		// Create a new slice and copy elements.
		// This is important to avoid modifying original slice data if it's not []any.
		count := val.Len()
		slice := make([]any, count)
		for i := 0; i < count; i++ {
			slice[i] = val.Index(i).Interface()
		}
		return slice, true
	}
	return nil, false
}

// Helper function convertToMap (not strictly needed if mergo handles it, but good for type safety before calling mergo)
// For this implementation, direct type assertion `value.(map[string]any)` is used in mergeValues.
// mergo itself can handle map[any]any to some extent, but map[string]any is common for JSON/YAML data.
// This function is not used in the current implementation above but kept for reference.
func convertToMap(value any) (map[string]any, bool) {
	if m, ok := value.(map[string]any); ok {
		return m, true
	}
	// Potentially handle map[any]any or structs here if needed,
	// but mergo might do some of this. For now, keep it simple.
	return nil, false
}
