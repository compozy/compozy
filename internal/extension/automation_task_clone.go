package extensionpkg

import (
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/network/participation"
)

func cloneAutomationTaskConfig(config *automationpkg.JobTaskConfig) *automationpkg.JobTaskConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	if config.Owner != nil {
		owner := *config.Owner
		cloned.Owner = &owner
	}
	cloned.NetworkParticipation = participation.CloneRequest(config.NetworkParticipation)
	return &cloned
}
