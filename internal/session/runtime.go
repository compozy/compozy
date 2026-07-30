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
	if normalized.Model != "" && normalized.Provider == "" {
		return RuntimeSelection{}, fmt.Errorf(
			"%w: provider is required when model is set",
			ErrInvalidRuntimeOverride,
		)
	}
	if normalized.ReasoningEffort != "" {
		if normalized.Provider == "" {
			return RuntimeSelection{}, fmt.Errorf(
				"%w: provider is required when reasoning_effort is set",
				ErrInvalidRuntimeOverride,
			)
		}
		if err := ValidateReasoningEffort(normalized.ReasoningEffort); err != nil {
			return RuntimeSelection{}, err
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
		selection = session.runtimeBindingSnapshot().selection
	}
	normalized, err := NormalizeRuntimeSelection(selection)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
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
