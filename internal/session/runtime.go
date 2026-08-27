package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

// RuntimeStatus reports the binding lifecycle between a logical session and
// its current ACP process.
type RuntimeStatus = store.SessionRuntimeStatus

const (
	RuntimeStatusUnbound       = store.SessionRuntimeUnbound
	RuntimeStatusBinding       = store.SessionRuntimeBinding
	RuntimeStatusReady         = store.SessionRuntimeReady
	RuntimeStatusReconfiguring = store.SessionRuntimeReconfiguring
	RuntimeStatusRecovering    = store.SessionRuntimeRecovering
	RuntimeStatusFailed        = store.SessionRuntimeFailed
)

// RuntimeTransitionStrategy identifies how the effective runtime changed at a
// prompt boundary.
type RuntimeTransitionStrategy = store.SessionRuntimeTransition

const (
	RuntimeTransitionNone               = store.SessionRuntimeTransitionNone
	RuntimeTransitionInitialBind        = store.SessionRuntimeTransitionInitialBind
	RuntimeTransitionLiveConfiguration  = store.SessionRuntimeTransitionLiveConfiguration
	RuntimeTransitionProcessReplacement = store.SessionRuntimeTransitionProcessReplacement
	RuntimeTransitionAutomaticRecovery  = store.SessionRuntimeTransitionAutomaticRecovery
)

// RuntimeSelection is the immutable runtime snapshot accepted with one prompt.
type RuntimeSelection struct {
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           speedpkg.Speed
	ACPOptions      []acp.SessionConfigOptionSelection
}

// NormalizeRuntimeSelection returns the canonical prompt-bound runtime input.
func NormalizeRuntimeSelection(selection RuntimeSelection) (RuntimeSelection, error) {
	normalized := RuntimeSelection{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           selection.Speed,
	}
	options, err := acp.NormalizeSessionConfigOptionSelections(selection.ACPOptions)
	if err != nil {
		return RuntimeSelection{}, fmt.Errorf("%w: %w", ErrInvalidRuntimeOverride, err)
	}
	normalized.ACPOptions = options
	if normalized.Speed == "" {
		normalized.Speed = speedpkg.SpeedNormal
	}
	parsedSpeed, err := speedpkg.Parse(string(normalized.Speed))
	if err != nil {
		return RuntimeSelection{}, fmt.Errorf("%w: %w", ErrInvalidRuntimeOverride, err)
	}
	normalized.Speed = parsedSpeed
	if normalized.Provider == "" {
		return RuntimeSelection{}, fmt.Errorf("%w: provider is required", ErrInvalidRuntimeOverride)
	}
	if normalized.ReasoningEffort != "" {
		if err := ValidateReasoningEffort(normalized.ReasoningEffort); err != nil {
			return RuntimeSelection{}, fmt.Errorf("%w: %w", ErrInvalidRuntimeOverride, err)
		}
	}
	return normalized, nil
}

func normalizePromptRuntimeSelection(
	session *Session,
	requested *RuntimeSelection,
) (*RuntimeSelection, error) {
	selection := RuntimeSelection{}
	if requested != nil {
		selection = *requested
	} else if session != nil {
		selected := session.selectedRuntimeSnapshot()
		if selected != nil {
			selection = *selected
		} else {
			selection = session.runtimeBindingSnapshot().selection
		}
	}
	normalized, err := NormalizeRuntimeSelection(selection)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (s *Session) selectedRuntimeSnapshot() *RuntimeSelection {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneRuntimeSelection(s.SelectedRuntime)
}

func runtimeSelectionsEqual(left RuntimeSelection, right RuntimeSelection) bool {
	return strings.TrimSpace(left.Provider) == strings.TrimSpace(right.Provider) &&
		strings.TrimSpace(left.Model) == strings.TrimSpace(right.Model) &&
		strings.TrimSpace(left.ReasoningEffort) == strings.TrimSpace(right.ReasoningEffort) &&
		left.Speed == right.Speed &&
		runtimeACPOptionSelectionsEqual(left.ACPOptions, right.ACPOptions)
}

func cloneRuntimeSelection(selection *RuntimeSelection) *RuntimeSelection {
	if selection == nil {
		return nil
	}
	cloned := *selection
	cloned.ACPOptions = acp.CloneSessionConfigOptionSelections(selection.ACPOptions)
	return &cloned
}

func storeSessionRuntimeSelection(selection *RuntimeSelection) *store.SessionRuntimeSelection {
	if selection == nil {
		return nil
	}
	return store.NormalizeSessionRuntimeSelection(&store.SessionRuntimeSelection{
		Provider:        selection.Provider,
		Model:           selection.Model,
		ReasoningEffort: selection.ReasoningEffort,
		Speed:           selection.Speed,
		ACPOptions:      storeSessionACPOptionSelections(selection.ACPOptions),
	})
}

func runtimeSelectionFromSessionStore(selection *store.SessionRuntimeSelection) *RuntimeSelection {
	if selection == nil {
		return nil
	}
	normalized := store.NormalizeSessionRuntimeSelection(selection)
	return &RuntimeSelection{
		Provider:        normalized.Provider,
		Model:           normalized.Model,
		ReasoningEffort: normalized.ReasoningEffort,
		Speed:           normalized.Speed,
		ACPOptions:      acpSessionACPOptionSelections(normalized.ACPOptions),
	}
}

func storeRuntimeSelection(selection *RuntimeSelection) store.SessionInputRuntime {
	if selection == nil {
		return store.SessionInputRuntime{}
	}
	return store.SessionInputRuntime{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           string(selection.Speed),
		ACPOptions:      storeSessionACPOptionSelections(selection.ACPOptions),
	}
}

func runtimeSelectionFromStore(selection store.SessionInputRuntime) *RuntimeSelection {
	if strings.TrimSpace(selection.Provider) == "" &&
		strings.TrimSpace(selection.Model) == "" &&
		strings.TrimSpace(selection.ReasoningEffort) == "" &&
		strings.TrimSpace(selection.Speed) == "" &&
		len(selection.ACPOptions) == 0 {
		return nil
	}
	return &RuntimeSelection{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           speedpkg.Speed(strings.TrimSpace(selection.Speed)),
		ACPOptions:      acpSessionACPOptionSelections(selection.ACPOptions),
	}
}

func storeSessionACPOptionSelections(
	selections []acp.SessionConfigOptionSelection,
) []store.SessionACPOptionSelection {
	if len(selections) == 0 {
		return nil
	}
	cloned := make([]store.SessionACPOptionSelection, 0, len(selections))
	for _, selection := range selections {
		candidate := store.SessionACPOptionSelection{
			ID:      strings.TrimSpace(selection.ID),
			ValueID: strings.TrimSpace(selection.ValueID),
		}
		if selection.BoolValue != nil {
			candidate.BoolValue = new(*selection.BoolValue)
		}
		cloned = append(cloned, candidate)
	}
	return store.NormalizeSessionACPOptionSelections(cloned)
}

func acpSessionACPOptionSelections(
	selections []store.SessionACPOptionSelection,
) []acp.SessionConfigOptionSelection {
	if len(selections) == 0 {
		return nil
	}
	cloned := make([]acp.SessionConfigOptionSelection, 0, len(selections))
	for _, selection := range selections {
		candidate := acp.SessionConfigOptionSelection{
			ID:      strings.TrimSpace(selection.ID),
			ValueID: strings.TrimSpace(selection.ValueID),
		}
		if selection.BoolValue != nil {
			candidate.BoolValue = new(*selection.BoolValue)
		}
		cloned = append(cloned, candidate)
	}
	return candidateACPOptionSelections(cloned)
}

func candidateACPOptionSelections(
	selections []acp.SessionConfigOptionSelection,
) []acp.SessionConfigOptionSelection {
	normalized, err := acp.NormalizeSessionConfigOptionSelections(selections)
	if err != nil {
		return acp.CloneSessionConfigOptionSelections(selections)
	}
	return normalized
}

func runtimeACPOptionSelectionsEqual(
	left []acp.SessionConfigOptionSelection,
	right []acp.SessionConfigOptionSelection,
) bool {
	left = candidateACPOptionSelections(left)
	right = candidateACPOptionSelections(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || left[index].ValueID != right[index].ValueID {
			return false
		}
		if (left[index].BoolValue == nil) != (right[index].BoolValue == nil) {
			return false
		}
		if left[index].BoolValue != nil && *left[index].BoolValue != *right[index].BoolValue {
			return false
		}
	}
	return true
}

func promptRuntimeFromSelection(selection RuntimeSelection) *acp.PromptRuntime {
	return &acp.PromptRuntime{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           selection.Speed,
		ACPOptions:      acp.CloneSessionConfigOptionSelections(selection.ACPOptions),
	}
}

func promptRuntimeFromSelectionPointer(selection *RuntimeSelection) *acp.PromptRuntime {
	if selection == nil {
		return nil
	}
	return promptRuntimeFromSelection(*selection)
}
