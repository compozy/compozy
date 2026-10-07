package extensionpkg

import (
	automationpkg "github.com/compozy/compozy/internal/automation"
)

func cloneAutomationTaskConfig(config *automationpkg.JobTaskConfig) *automationpkg.JobTaskConfig {
	if config == nil {
		return nil
	}
	cloned := *config
	if config.Owner != nil {
		cloned.Owner = new(*config.Owner)
	}
	return &cloned
}
