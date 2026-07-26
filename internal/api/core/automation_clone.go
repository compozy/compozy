package core

import (
	"fmt"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func cloneAutomationJobTaskConfig(config *automationpkg.JobTaskConfig) (*automationpkg.JobTaskConfig, error) {
	if config == nil {
		return nil, nil
	}
	cloned := *config
	cloned.Title = strings.TrimSpace(cloned.Title)
	cloned.Description = strings.TrimSpace(cloned.Description)
	networkParticipation, err := automationpkg.NormalizeDirectTaskParticipation(config.NetworkParticipation)
	if err != nil {
		return nil, fmt.Errorf("normalize Automation task participation: %w", err)
	}
	cloned.NetworkParticipation = participation.CloneRequest(networkParticipation)
	if config.Owner != nil {
		owner := *config.Owner
		owner.Kind = taskpkg.OwnerKind(strings.TrimSpace(string(owner.Kind)))
		owner.Ref = strings.TrimSpace(owner.Ref)
		cloned.Owner = &owner
	}
	return &cloned, nil
}
