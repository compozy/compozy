package ref

import (
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

// MergeMode merges reference and inline values, with reference winning conflicts.
type MergeMode struct{}

func (m *MergeMode) Merge(refValue, inlineValue any) (any, error) {
	if _, ok := refValue.([]any); ok {
		return refValue, nil
	}
	refMap, refOk := refValue.(map[string]any)
	inlineMap, inlineOk := inlineValue.(map[string]any)
	if refOk && inlineOk {
		result := make(map[string]any)
		for k, v := range inlineMap {
			result[k] = v
		}
		for k, v := range refMap {
			result[k] = v
		}
		return result, nil
	}
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
