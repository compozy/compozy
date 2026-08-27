package store

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// SessionACPOptionSelection is one typed provider option captured by a
// session or prompt. Exactly one of ValueID and BoolValue must be set.
type SessionACPOptionSelection struct {
	ID        string `json:"id"`
	ValueID   string `json:"value_id,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty"`
}

// Normalize trims values, copies boolean pointers, and orders selections by ID.
func NormalizeSessionACPOptionSelections(
	selections []SessionACPOptionSelection,
) []SessionACPOptionSelection {
	if len(selections) == 0 {
		return nil
	}
	normalized := make([]SessionACPOptionSelection, 0, len(selections))
	for _, selection := range selections {
		candidate := SessionACPOptionSelection{
			ID:      strings.TrimSpace(selection.ID),
			ValueID: strings.TrimSpace(selection.ValueID),
		}
		if selection.BoolValue != nil {
			candidate.BoolValue = new(*selection.BoolValue)
		}
		normalized = append(normalized, candidate)
	}
	slices.SortFunc(normalized, func(left, right SessionACPOptionSelection) int {
		return cmp.Compare(left.ID, right.ID)
	})
	return normalized
}

// CloneSessionACPOptionSelections returns an ownership-safe copy of selections.
func CloneSessionACPOptionSelections(
	selections []SessionACPOptionSelection,
) []SessionACPOptionSelection {
	return NormalizeSessionACPOptionSelections(selections)
}

// ValidateSessionACPOptionSelections checks the persisted typed option shape.
func ValidateSessionACPOptionSelections(selections []SessionACPOptionSelection) error {
	seen := make(map[string]struct{}, len(selections))
	for index, selection := range selections {
		id := strings.TrimSpace(selection.ID)
		if id == "" {
			return fmt.Errorf("store: session ACP option %d ID is required", index)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("store: session ACP option %q is selected more than once", id)
		}
		seen[id] = struct{}{}
		if (strings.TrimSpace(selection.ValueID) == "") == (selection.BoolValue == nil) {
			return errors.New(
				"store: session ACP option selection requires exactly one value_id or bool_value",
			)
		}
	}
	return nil
}

// ValidateSessionInputRuntime checks the typed options carried with queued or
// admitted prompt input without imposing provider-specific capability rules.
func ValidateSessionInputRuntime(runtime SessionInputRuntime) error {
	if err := ValidateSessionACPOptionSelections(runtime.ACPOptions); err != nil {
		return fmt.Errorf("store: validate session input ACP options: %w", err)
	}
	return nil
}
