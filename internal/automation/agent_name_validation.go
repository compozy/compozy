package automation

import (
	"fmt"
	"regexp"
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
	if trigger.TargetKind.Normalize() == TargetKindAgentInstance &&
		agentInstanceIDPattern.MatchString(strings.TrimSpace(trigger.AgentName)) {
		return compozyconfig.ValidationError{
			Path: nestedPath(path, "agent_name"),
			Message: fmt.Sprintf(
				"%q is an agent instance id; target an always-on agent by its agent name, such as \"triage\"",
				strings.TrimSpace(trigger.AgentName),
			),
		}
	}
	return nil
}

// agentInstanceIDPattern matches the IDs the store issues for agent instances (`ainst-` and 16 hex
// digits). An `agent_instance` trigger names the agent; its instance resolves per delivery.
var agentInstanceIDPattern = regexp.MustCompile(`^ainst-[0-9a-f]{16}$`)
