package store

import (
	"fmt"
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

// SessionRuntimeSelection is the durable runtime preference for future prompts.
type SessionRuntimeSelection struct {
	Provider        string                      `json:"provider"`
	Model           string                      `json:"model,omitempty"`
	ReasoningEffort string                      `json:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed              `json:"speed,omitempty"`
	ACPOptions      []SessionACPOptionSelection `json:"acp_options,omitempty"`
}

// SessionRuntimeSelectionState keeps a selected runtime and its revision together.
type SessionRuntimeSelectionState struct {
	Selected *SessionRuntimeSelection `json:"selected,omitempty"`
	Revision int64                    `json:"revision"`
}

// NewSessionRuntimeSelectionState returns canonical persisted selection state.
func NewSessionRuntimeSelectionState(
	selection *SessionRuntimeSelection,
	revision int64,
) *SessionRuntimeSelectionState {
	if selection == nil && revision == 0 {
		return nil
	}
	return &SessionRuntimeSelectionState{
		Selected: CloneSessionRuntimeSelection(selection),
		Revision: revision,
	}
}

// SessionRuntimeSelectionStateValues returns independent values from persisted selection state.
func SessionRuntimeSelectionStateValues(
	state *SessionRuntimeSelectionState,
) (*SessionRuntimeSelection, int64) {
	if state == nil {
		return nil, 0
	}
	return CloneSessionRuntimeSelection(state.Selected), state.Revision
}

// ValidateSessionRuntimeSelectionState ensures persisted selection state is well formed.
func ValidateSessionRuntimeSelectionState(state *SessionRuntimeSelectionState) error {
	selection, revision := SessionRuntimeSelectionStateValues(state)
	return ValidateSessionRuntimeSelection(selection, revision)
}

// NormalizeSessionRuntimeSelection returns an independent canonical selection.
func NormalizeSessionRuntimeSelection(selection *SessionRuntimeSelection) *SessionRuntimeSelection {
	if selection == nil {
		return nil
	}
	normalized := &SessionRuntimeSelection{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           selection.Speed,
		ACPOptions:      NormalizeSessionACPOptionSelections(selection.ACPOptions),
	}
	if normalized.Speed == "" {
		normalized.Speed = speedpkg.SpeedNormal
	}
	return normalized
}

// ValidateSessionRuntimeSelection ensures a persisted preference and revision are well formed.
func ValidateSessionRuntimeSelection(selection *SessionRuntimeSelection, revision int64) error {
	if revision < 0 {
		return fmt.Errorf("store: runtime selection revision must not be negative")
	}
	if selection == nil {
		return nil
	}
	normalized := NormalizeSessionRuntimeSelection(selection)
	if normalized.Provider == "" {
		return fmt.Errorf("store: selected runtime provider is required")
	}
	if _, err := speedpkg.Parse(string(normalized.Speed)); err != nil {
		return fmt.Errorf("store: validate selected runtime speed: %w", err)
	}
	if err := ValidateSessionACPOptionSelections(normalized.ACPOptions); err != nil {
		return fmt.Errorf("store: validate selected runtime ACP options: %w", err)
	}
	return nil
}

// CloneSessionRuntimeSelection returns an independent canonical selection.
func CloneSessionRuntimeSelection(selection *SessionRuntimeSelection) *SessionRuntimeSelection {
	return NormalizeSessionRuntimeSelection(selection)
}
