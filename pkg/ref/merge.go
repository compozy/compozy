package ref

import (
	"maps"

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

// ReplaceMode replaces the inline value with the reference value.
type ReplaceMode struct{}

func (m *ReplaceMode) Merge(refValue, _ any) (any, error) {
	return refValue, nil
}

// -----------------------------------------------------------------------------
// Append Mode
// -----------------------------------------------------------------------------

// AppendMode appends reference and inline arrays.
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

// MergeMode merges reference and inline values, with inline winning conflicts.
type MergeMode struct{}

func (m *MergeMode) Merge(refValue, inlineValue any) (any, error) {
	// For arrays, replace (as per spec)
	if _, ok := refValue.([]any); ok {
		return refValue, nil
	}

	// For maps, deep merge with inline winning conflicts
	refMap, refOk := refValue.(map[string]any)
	inlineMap, inlineOk := inlineValue.(map[string]any)
	if refOk && inlineOk {
		result := make(map[string]any)
		maps.Copy(result, refMap)
		if err := mergo.Merge(&result, inlineMap, mergo.WithOverride); err != nil {
			return nil, errors.Wrap(err, "failed to merge maps")
		}
		return result, nil
	}

	// For other types, inline wins
	return inlineValue, nil
}

// -----------------------------------------------------------------------------
// Apply Merge Mode
// -----------------------------------------------------------------------------

// ApplyMergeMode applies the merge mode to combine reference and inline values.
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
