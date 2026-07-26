package extensionpkg

import (
	"maps"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
)

func cloneHostAPIAutomationLoopTarget(source *automationpkg.LoopTarget) *automationpkg.LoopTarget {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Inputs = maps.Clone(source.Inputs)
	cloned.InputMapping = maps.Clone(source.InputMapping)
	cloned.NetworkParticipation = participation.CloneRequest(source.NetworkParticipation)
	return &cloned
}
