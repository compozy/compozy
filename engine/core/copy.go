package core

import (
	"fmt"

	"github.com/mohae/deepcopy"
)

// DeepCopy creates a deep copy of the supplied value.
// It has special handling for Input / Output (and their pointer forms)
// so that the copy retains the correct concrete type instead of devolving
// into a plain map returned by the deepcopy library.
//
// For every other type the helper falls back to deepcopy.Copy.
func DeepCopy[T any](v T) (T, error) {
	var zero T // zero value used for early returns

	switch src := any(v).(type) {

	// ------------------------------------------------------------------
	// Direct Input / Output values
	// ------------------------------------------------------------------
	case Input:
		copied := deepcopy.Copy(map[string]any(src)).(map[string]any)
		dst := Input(copied)
		return any(dst).(T), nil

	case Output:
		copied := deepcopy.Copy(map[string]any(src)).(map[string]any)
		dst := Output(copied)
		return any(dst).(T), nil

	// ------------------------------------------------------------------
	// *Input / *Output pointer values
	// ------------------------------------------------------------------
	case *Input:
		if src == nil {
			return zero, nil
		}
		copied := deepcopy.Copy(map[string]any(*src)).(map[string]any)
		dst := Input(copied)
		return any(&dst).(T), nil

	case *Output:
		if src == nil {
			return zero, nil
		}
		copied := deepcopy.Copy(map[string]any(*src)).(map[string]any)
		dst := Output(copied)
		return any(&dst).(T), nil

	// ------------------------------------------------------------------
	// Everything else – delegate to library
	// ------------------------------------------------------------------
	default:
		copied := deepcopy.Copy(v)
		result, ok := copied.(T)
		if !ok {
			return zero, fmt.Errorf("failed to cast copied value to type %T", zero)
		}
		return result, nil
	}
}
