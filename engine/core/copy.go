package core

import (
	"fmt"

	"github.com/mohae/deepcopy"
)

// deepCopyMap returns a deep copy of the provided map[string]any.
//
// If the underlying copy cannot be asserted back to map[string]any an error is returned.
func deepCopyMap(m map[string]any) (map[string]any, error) {
	copiedInterface := deepcopy.Copy(m)
	copied, ok := copiedInterface.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("failed to copy map")
	}
	return copied, nil
}

// DeepCopy creates a deep copy of the supplied value.
// It has special handling for Input / Output (and their pointer forms)
// so that the copy retains the correct concrete type instead of devolving
// into a plain map returned by the deepcopy library.
//
// DeepCopy returns a deep copy of v, preserving concrete Input/Output types (and their pointer forms).
//
// It handles the following special cases:
//   - Input and Output (non-pointer): copied via an internal map-based copy and
//     reconstructed as the same concrete type.
//   - *Input and *Output: nil-checked, copied by dereferencing and reconstructed as a
//     pointer to the copied concrete type.
//
// For all other types it falls back to a generic deep copy (using deepcopy.Copy).
//
// On success the copied value of type T is returned. On failure the zero value of T and a non-nil error are returned.
// Note: nil pointer Input/Output values are treated as absent and result in the zero value of T with a nil error.
func DeepCopy[T any](v T) (T, error) {
	var zero T // zero value used for early returns

	switch src := any(v).(type) {
	case Input:
		return deepCopyInput(src, zero)
	case Output:
		return deepCopyOutput(src, zero)
	case *Input:
		return deepCopyInputPtr(src, zero)
	case *Output:
		return deepCopyOutputPtr(src, zero)
	default:
		return deepCopyGeneric(v, zero)
	}
}

// deepCopyInput deep-copies a non-pointer Input value and returns it as type T.
//
// If src is nil this returns the provided zero value. The function deep-copies
// the underlying map via deepCopyMap, reconstructs an Input from the copied map,
// and attempts to convert that value to T. Returns an error if the underlying
// map copy fails or if the converted value cannot be asserted to T.
func deepCopyInput[T any](src Input, zero T) (T, error) {
	// Check if the Input (which is a map) is nil
	if src == nil {
		return zero, nil
	}
	copied, err := deepCopyMap(map[string]any(src))
	if err != nil {
		return zero, fmt.Errorf("failed to copy Input type: %w", err)
	}
	dst := Input(copied)
	result, ok := any(dst).(T)
	if !ok {
		return zero, fmt.Errorf("failed to cast Input to type %T", zero)
	}
	return result, nil
}

// deepCopyOutput deep copies an Output (map[string]any) value and returns it as type T.
//
// If src is nil this returns the zero value provided and no error. It uses deepCopyMap
// to clone the underlying map and converts the result back to Output before asserting
// the requested generic type T. Returns an error if the map copy fails or if the
// final type assertion to T is not possible.
func deepCopyOutput[T any](src Output, zero T) (T, error) {
	// Check if the Output (which is a map) is nil
	if src == nil {
		return zero, nil
	}
	copied, err := deepCopyMap(map[string]any(src))
	if err != nil {
		return zero, fmt.Errorf("failed to copy Output type: %w", err)
	}
	dst := Output(copied)
	result, ok := any(dst).(T)
	if !ok {
		return zero, fmt.Errorf("failed to cast Output to type %T", zero)
	}
	return result, nil
}

// deepCopyInputPtr deep copies a *Input into the requested generic type T.
// If src is nil or *src is nil it returns the provided zero value and no error.
// The function performs a deep copy of the underlying map[string]any via deepCopyMap,
// reconstructs an Input from the copied map, and attempts to return a pointer to that
// Input cast to T. It returns an error if the map copy fails or if the final cast
// to T is unsuccessful.
func deepCopyInputPtr[T any](src *Input, zero T) (T, error) {
	if src == nil {
		return zero, nil
	}
	// Also check if the pointed-to Input is nil
	if *src == nil {
		return zero, nil
	}
	copied, err := deepCopyMap(map[string]any(*src))
	if err != nil {
		return zero, fmt.Errorf("failed to copy *Input type: %w", err)
	}
	dst := Input(copied)
	result, ok := any(&dst).(T)
	if !ok {
		return zero, fmt.Errorf("failed to cast *Input to type %T", zero)
	}
	return result, nil
}

// deepCopyOutputPtr deeply copies a *Output and returns the result as type T.
// If src is nil or *src is nil the supplied zero value is returned with no error.
// On success returns a pointer to a new Output whose underlying map has been deep-copied.
// Returns an error if the map copy fails or if the copied pointer cannot be cast to T.
func deepCopyOutputPtr[T any](src *Output, zero T) (T, error) {
	if src == nil {
		return zero, nil
	}
	// Also check if the pointed-to Output is nil
	if *src == nil {
		return zero, nil
	}
	copied, err := deepCopyMap(map[string]any(*src))
	if err != nil {
		return zero, fmt.Errorf("failed to copy *Output type: %w", err)
	}
	dst := Output(copied)
	result, ok := any(&dst).(T)
	if !ok {
		return zero, fmt.Errorf("failed to cast *Output to type %T", zero)
	}
	return result, nil
}

// deepCopyGeneric creates a deep copy of v using github.com/mohae/deepcopy and returns it as type T.
// It is used for values that don't require special Input/Output handling. If the copied value cannot
// be asserted back to T the function returns the provided zero value and an error.
func deepCopyGeneric[T any](v T, zero T) (T, error) {
	copied := deepcopy.Copy(v)
	result, ok := copied.(T)
	if !ok {
		return zero, fmt.Errorf("failed to cast copied value to type %T", zero)
	}
	return result, nil
}
