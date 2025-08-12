package core

import (
	"fmt"

	"github.com/mohae/deepcopy"
)

// deepCopyMap performs a deep copy of a map and returns the result
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
// For every other type the helper falls back to deepcopy.Copy.
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

// deepCopyInput handles deep copying of Input type
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

// deepCopyOutput handles deep copying of Output type
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

// deepCopyInputPtr handles deep copying of *Input type
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

// deepCopyOutputPtr handles deep copying of *Output type
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

// deepCopyGeneric handles deep copying of generic types
func deepCopyGeneric[T any](v T, zero T) (T, error) {
	copied := deepcopy.Copy(v)
	result, ok := copied.(T)
	if !ok {
		return zero, fmt.Errorf("failed to cast copied value to type %T", zero)
	}
	return result, nil
}
