package core

import (
	"fmt"
	"maps"

	"dario.cat/mergo"
	"github.com/mohae/deepcopy"
)

// Merge combines two maps, with source values overriding destination values.
// Slice values are appended rather than replaced.
func Merge[D, S ~map[string]any](dst D, src S, kind string) (D, error) {
	var zero D
	dstClone := CloneMap(dst)
	srcClone := CloneMap(src)
	if len(srcClone) > 0 {
		if err := mergo.Merge(&dstClone, srcClone, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
			return zero, fmt.Errorf("failed to merge %s: %w", kind, err)
		}
	}
	return dstClone, nil
}

// CloneMap creates a shallow copy of any map type with comparable keys.
// This is useful for copying configuration maps, metadata, and other map structures
// where you need to modify the copy without affecting the original.
// Returns an empty initialized map when src is nil to prevent nil map panics.
func CloneMap[K comparable, V any](src map[K]V) map[K]V {
	if src == nil {
		return make(map[K]V)
	}
	return maps.Clone(src)
}

// CopyMaps safely merges multiple maps into a new map, with later maps
// overriding earlier ones. Handles nil maps gracefully by skipping them.
// Returns an empty initialized map if all inputs are nil.
func CopyMaps[K comparable, V any](srcs ...map[K]V) map[K]V {
	result := make(map[K]V)
	for _, src := range srcs {
		if src != nil {
			maps.Copy(result, src)
		}
	}
	return result
}

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
		return DeepCopyInput(src, zero)
	case Output:
		return DeepCopyOutput(src, zero)
	case *Input:
		return DeepCopyInputPtr(src, zero)
	case *Output:
		return DeepCopyOutputPtr(src, zero)
	default:
		return DeepCopyGeneric(v, zero)
	}
}

// DeepCopyInput deep-copies a non-pointer Input value and returns it as type T.
//
// If src is nil this returns the provided zero value. The function deep-copies
// the underlying map via deepCopyMap, reconstructs an Input from the copied map,
// and attempts to convert that value to T. Returns an error if the underlying
// map copy fails or if the converted value cannot be asserted to T.
func DeepCopyInput[T any](src Input, zero T) (T, error) {
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

// DeepCopyOutput deep copies an Output (map[string]any) value and returns it as type T.
//
// If src is nil this returns the zero value provided and no error. It uses deepCopyMap
// to clone the underlying map and converts the result back to Output before asserting
// the requested generic type T. Returns an error if the map copy fails or if the
// final type assertion to T is not possible.
func DeepCopyOutput[T any](src Output, zero T) (T, error) {
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

// DeepCopyInputPtr deep copies a *Input into the requested generic type T.
// If src is nil or *src is nil it returns the provided zero value and no error.
// The function performs a deep copy of the underlying map[string]any via deepCopyMap,
// reconstructs an Input from the copied map, and attempts to return a pointer to that
// Input cast to T. It returns an error if the map copy fails or if the final cast
// to T is unsuccessful.
func DeepCopyInputPtr[T any](src *Input, zero T) (T, error) {
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

// DeepCopyOutputPtr deeply copies a *Output and returns the result as type T.
// If src is nil or *src is nil the supplied zero value is returned with no error.
// On success returns a pointer to a new Output whose underlying map has been deep-copied.
// Returns an error if the map copy fails or if the copied pointer cannot be cast to T.
func DeepCopyOutputPtr[T any](src *Output, zero T) (T, error) {
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

// DeepCopyGeneric creates a deep copy of v using github.com/mohae/deepcopy and returns it as type T.
// It is used for values that don't require special Input/Output handling. If the copied value cannot
// be asserted back to T the function returns the provided zero value and an error.
func DeepCopyGeneric[T any](v T, zero T) (T, error) {
	copied := deepcopy.Copy(v)
	result, ok := copied.(T)
	if !ok {
		return zero, fmt.Errorf("failed to cast copied value to type %T", zero)
	}
	return result, nil
}
