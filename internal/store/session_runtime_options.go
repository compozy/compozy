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

// SessionRuntimeDetails keeps optional ACP selections and recovery metadata
// compact while preserving their flat session metadata JSON fields.
type SessionRuntimeDetails struct {
	ACPOptions      []SessionACPOptionSelection `json:"acp_options,omitempty"`
	RuntimeRecovery *SessionRuntimeRecovery     `json:"runtime_recovery,omitempty"`
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

// ACPOptionsValue returns the persisted session option selections.
func (m SessionMeta) ACPOptionsValue() []SessionACPOptionSelection {
	if m.SessionRuntimeDetails == nil {
		return nil
	}
	return m.ACPOptions
}

// SetACPOptions stores an ownership-safe session option selection set.
func (m *SessionMeta) SetACPOptions(selections []SessionACPOptionSelection) {
	setSessionRuntimeACPOptions(&m.SessionRuntimeDetails, selections)
}

// ACPOptionsValue returns the indexed session option selections.
func (s SessionInfo) ACPOptionsValue() []SessionACPOptionSelection {
	if s.SessionRuntimeDetails == nil {
		return nil
	}
	return s.ACPOptions
}

// SetACPOptions stores an ownership-safe indexed option selection set.
func (s *SessionInfo) SetACPOptions(selections []SessionACPOptionSelection) {
	setSessionRuntimeACPOptions(&s.SessionRuntimeDetails, selections)
}

// RuntimeRecoveryValue returns independent persisted recovery metadata.
func (m SessionMeta) RuntimeRecoveryValue() *SessionRuntimeRecovery {
	if m.SessionRuntimeDetails == nil {
		return nil
	}
	return CloneSessionRuntimeRecovery(m.RuntimeRecovery)
}

// SetRuntimeRecovery stores independent persisted recovery metadata.
func (m *SessionMeta) SetRuntimeRecovery(recovery *SessionRuntimeRecovery) {
	setSessionRuntimeRecovery(&m.SessionRuntimeDetails, recovery)
}

// RuntimeRecoveryValue returns independent indexed recovery metadata.
func (s SessionInfo) RuntimeRecoveryValue() *SessionRuntimeRecovery {
	if s.SessionRuntimeDetails == nil {
		return nil
	}
	return CloneSessionRuntimeRecovery(s.RuntimeRecovery)
}

// SetRuntimeRecovery stores independent indexed recovery metadata.
func (s *SessionInfo) SetRuntimeRecovery(recovery *SessionRuntimeRecovery) {
	setSessionRuntimeRecovery(&s.SessionRuntimeDetails, recovery)
}

func setSessionRuntimeACPOptions(
	state **SessionRuntimeDetails,
	selections []SessionACPOptionSelection,
) {
	cloned := CloneSessionACPOptionSelections(selections)
	if *state == nil {
		if len(cloned) == 0 {
			return
		}
		*state = &SessionRuntimeDetails{}
	}
	(*state).ACPOptions = cloned
	clearEmptySessionRuntimeDetails(state)
}

func setSessionRuntimeRecovery(
	state **SessionRuntimeDetails,
	recovery *SessionRuntimeRecovery,
) {
	cloned := CloneSessionRuntimeRecovery(recovery)
	if *state == nil {
		if cloned == nil {
			return
		}
		*state = &SessionRuntimeDetails{}
	}
	(*state).RuntimeRecovery = cloned
	clearEmptySessionRuntimeDetails(state)
}

func clearEmptySessionRuntimeDetails(state **SessionRuntimeDetails) {
	if *state != nil && len((*state).ACPOptions) == 0 && (*state).RuntimeRecovery == nil {
		*state = nil
	}
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
