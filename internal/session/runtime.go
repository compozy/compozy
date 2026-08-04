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
)

// RuntimeSelection is the immutable runtime snapshot accepted with one prompt.
type RuntimeSelection struct {
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           speedpkg.Speed
}

// NormalizeRuntimeSelection returns the canonical prompt-bound runtime input.
func NormalizeRuntimeSelection(selection RuntimeSelection) (RuntimeSelection, error) {
	normalized := RuntimeSelection{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           selection.Speed,
	}
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
		left.Speed == right.Speed
}

func cloneRuntimeSelection(selection *RuntimeSelection) *RuntimeSelection {
	if selection == nil {
		return nil
	}
	cloned := *selection
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
	}
}

func runtimeSelectionFromStore(selection store.SessionInputRuntime) *RuntimeSelection {
	if strings.TrimSpace(selection.Provider) == "" &&
		strings.TrimSpace(selection.Model) == "" &&
		strings.TrimSpace(selection.ReasoningEffort) == "" &&
		strings.TrimSpace(selection.Speed) == "" {
		return nil
	}
	return &RuntimeSelection{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           speedpkg.Speed(strings.TrimSpace(selection.Speed)),
	}
}

func promptRuntimeFromSelection(selection RuntimeSelection) *acp.PromptRuntime {
	return &acp.PromptRuntime{
		Provider:        strings.TrimSpace(selection.Provider),
		Model:           strings.TrimSpace(selection.Model),
		ReasoningEffort: strings.TrimSpace(selection.ReasoningEffort),
		Speed:           selection.Speed,
	}
}

func promptRuntimeFromSelectionPointer(selection *RuntimeSelection) *acp.PromptRuntime {
	if selection == nil {
		return nil
	}
	return promptRuntimeFromSelection(*selection)
}
