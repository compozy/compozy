package daemon

import (
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
)

func loopACPOptionsForSession(
	options []looppkg.ACPOptionSelection,
) []acp.SessionConfigOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]acp.SessionConfigOptionSelection, 0, len(options))
	for _, option := range options {
		candidate := acp.SessionConfigOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			candidate.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, candidate)
	}
	return converted
}

func loopACPOptionsFromSession(
	options []acp.SessionConfigOptionSelection,
) []looppkg.ACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]looppkg.ACPOptionSelection, 0, len(options))
	for _, option := range options {
		candidate := looppkg.ACPOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			candidate.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, candidate)
	}
	return converted
}

func sessionACPOptionsFromStore(
	options []store.SessionACPOptionSelection,
) []acp.SessionConfigOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]acp.SessionConfigOptionSelection, 0, len(options))
	for _, option := range options {
		candidate := acp.SessionConfigOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			candidate.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, candidate)
	}
	return converted
}

func loopACPOptionsFromStore(
	options []store.SessionACPOptionSelection,
) []looppkg.ACPOptionSelection {
	return loopACPOptionsFromSession(sessionACPOptionsFromStore(options))
}

func loopACPOptionsMatchStore(
	requested []looppkg.ACPOptionSelection,
	persisted []store.SessionACPOptionSelection,
) bool {
	left := store.NormalizeSessionACPOptionSelections(storeACPOptionsFromLoop(requested))
	right := store.NormalizeSessionACPOptionSelections(persisted)
	return slices.EqualFunc(left, right, func(a, b store.SessionACPOptionSelection) bool {
		if a.ID != b.ID || a.ValueID != b.ValueID || (a.BoolValue == nil) != (b.BoolValue == nil) {
			return false
		}
		return a.BoolValue == nil || *a.BoolValue == *b.BoolValue
	})
}

func storeACPOptionsFromLoop(
	options []looppkg.ACPOptionSelection,
) []store.SessionACPOptionSelection {
	if len(options) == 0 {
		return nil
	}
	converted := make([]store.SessionACPOptionSelection, 0, len(options))
	for _, option := range options {
		candidate := store.SessionACPOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			candidate.BoolValue = new(*option.BoolValue)
		}
		converted = append(converted, candidate)
	}
	return converted
}
