package core

import (
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func cloneAutomationJobTaskConfig(config *automationpkg.JobTaskConfig) *automationpkg.JobTaskConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	cloned.Title = strings.TrimSpace(cloned.Title)
	cloned.Description = strings.TrimSpace(cloned.Description)
	if config.Owner != nil {
		owner := *config.Owner
		owner.Kind = taskpkg.OwnerKind(strings.TrimSpace(string(owner.Kind)))
		owner.Ref = strings.TrimSpace(owner.Ref)
		cloned.Owner = &owner
	}
	return &cloned
}
