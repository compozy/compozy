package extensionpkg

import (
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
)

func cloneBundleTaskConfig(config *automationpkg.JobTaskConfig) *automationpkg.JobTaskConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	if config.Owner != nil {
		owner := *config.Owner
		cloned.Owner = &owner
	}
	cloned.NetworkParticipation = cloneBundleParticipationRequest(config.NetworkParticipation)
	return &cloned
}

func cloneBundleParticipationRequest(request *participation.Request) *participation.Request {
	return participation.CloneRequest(request)
}
