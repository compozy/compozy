package dsl

import (
	"github.com/compozy/compozy/internal/runtimeoption"
)

// ACPOptionSelection identifies one provider-advertised ACP option value.
// Exactly one of ValueID or BoolValue must be set.
type ACPOptionSelection = runtimeoption.Selection

// NormalizeACPOptionSelections validates, copies, and sorts ACP selections by ID.
func NormalizeACPOptionSelections(selections []ACPOptionSelection) ([]ACPOptionSelection, error) {
	return runtimeoption.NormalizeSelections("acp_options", selections)
}

// NormalizeACPOptionSelectionsAt validates selections while retaining their owning path.
func NormalizeACPOptionSelectionsAt(
	path string,
	selections []ACPOptionSelection,
) ([]ACPOptionSelection, error) {
	return runtimeoption.NormalizeSelections(path, selections)
}

// CloneACPOptionSelections returns an ownership-safe copy of ACP selections.
func CloneACPOptionSelections(selections []ACPOptionSelection) []ACPOptionSelection {
	return runtimeoption.CloneSelections(selections)
}

// CanonicalACPOptionSelections returns a sorted ownership-safe copy.
func CanonicalACPOptionSelections(selections []ACPOptionSelection) []ACPOptionSelection {
	return runtimeoption.CanonicalSelections(selections)
}

// MergeACPOptionSelections overlays typed values by canonical option ID.
func MergeACPOptionSelections(
	base []ACPOptionSelection,
	overlay []ACPOptionSelection,
) ([]ACPOptionSelection, []string) {
	return runtimeoption.MergeSelections(base, overlay)
}
