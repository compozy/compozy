package extensionpkg

import (
	"maps"

	automationpkg "github.com/compozy/compozy/internal/automation"
)

func cloneHostAPIAutomationLoopTarget(source *automationpkg.LoopTarget) *automationpkg.LoopTarget {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Inputs = maps.Clone(source.Inputs)
	cloned.InputMapping = maps.Clone(source.InputMapping)
	return &cloned
}
