package compozysdk

import (
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/sdk/go/contracts"
)

// ExtensionCommandGroupSpec declares a presentation-only command group.
type ExtensionCommandGroupSpec = contracts.ExtensionCommandGroupSpec

// CommandGroup declares help metadata for a one-segment command group.
// Command tree validation remains authoritative in the Compozy build and load paths.
func (e *Extension) CommandGroup(path, summary string) error {
	if e == nil {
		return NewInternalError("extension is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.initialized {
		return NewInvalidParamsError("extension registration is closed after initialize", nil)
	}
	e.commandGroups = append(e.commandGroups, contracts.ExtensionCommandGroupSpec{
		Path: strings.TrimSpace(path), Summary: strings.TrimSpace(summary),
	})
	return nil
}

func cloneSDKCommandSpec(src *ExtensionCommandSpec) *ExtensionCommandSpec {
	if src == nil {
		return nil
	}
	cloned := *src
	cloned.Flags = maps.Clone(src.Flags)
	return &cloned
}

func (e *Extension) commandGroupsLocked() []contracts.ExtensionCommandGroupSpec {
	return slices.Clone(e.commandGroups)
}
