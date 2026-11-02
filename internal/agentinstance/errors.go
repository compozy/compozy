package agentinstance

import (
	"errors"
	"fmt"
)

// ErrorMessage describes a domain failure without changing its stable code.
func ErrorMessage(err error, activeLimit int) string {
	if errors.Is(err, ErrLimit) {
		if activeLimit > 0 {
			return fmt.Sprintf(
				"You have %d active stays-on agents. Pause one or raise `agents.stays_on.max_active_instances`.",
				activeLimit,
			)
		}
		return "The active stays-on agent limit has been reached. Pause one or raise `agents.stays_on.max_active_instances`."
	}
	for _, item := range []struct {
		domain  error
		message string
	}{
		{ErrNotStaysOn, "This agent is on-demand. Only stays-on agents have instances."},
		{ErrForbidden, "An agent can only act on its own instance, within its limits."},
		{ErrDepth, "An instance created by an agent can't create more instances."},
		{ErrWorkspaceAccessDenied, "This agent can't reach that workspace."},
		{ErrExists, "This agent already runs in that scope."},
		{ErrSessionNotRoot, "Only root sessions can be adopted."},
		{ErrScopeMismatch, "This session can't be adopted by this agent and profile."},
		{ErrDefinitionMissing, "This agent's definition is gone. Reinstall it to resume. Its memory is kept."},
		{ErrStarting, "This agent is already starting there."},
		{ErrPaused, "This agent is paused. Dry runs still work."},
		{ErrNotFound, "This agent instance was not found."},
	} {
		if errors.Is(err, item.domain) {
			return item.message
		}
	}
	return ""
}
