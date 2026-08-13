package automation

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// ValidateJobAgentName validates an agent-targeted job reference at the owning
// dynamic automation boundary.
func ValidateJobAgentName(job Job, path string) error {
	if job.IsLoopTarget() || strings.TrimSpace(job.AgentName) == "" {
		return nil
	}
	if err := compozyconfig.ValidateAgentName(job.AgentName); err != nil {
		return compozyconfig.ValidationError{
			Path:    nestedPath(path, "agent_name"),
			Message: err.Error(),
		}
	}
	return nil
}

// ValidateTriggerAgentName validates an agent-targeted trigger reference at
// the owning dynamic automation boundary.
func ValidateTriggerAgentName(trigger Trigger, path string) error {
	if trigger.IsLoopTarget() || strings.TrimSpace(trigger.AgentName) == "" {
		return nil
	}
	if err := compozyconfig.ValidateAgentName(trigger.AgentName); err != nil {
		return compozyconfig.ValidationError{
			Path:    nestedPath(path, "agent_name"),
			Message: err.Error(),
		}
	}
	return nil
}
