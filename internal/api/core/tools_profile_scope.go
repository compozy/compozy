package core

import (
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

// resolveOperatorToolScope builds a privileged projection in the selected profile.
func (h *BaseHandlers) resolveOperatorToolScope(c *gin.Context) (toolspkg.Scope, error) {
	selection, err := h.resolveProfileReadSelection(c)
	if err != nil {
		return toolspkg.Scope{}, err
	}
	if selection.Scope.AllProfiles {
		return toolspkg.Scope{}, &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "tools require exactly one profile",
			Action:  "choose the profile that owns the tool catalog",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	return toolspkg.Scope{
		ProfileID:   strings.TrimSpace(selection.Scope.ProfileID),
		WorkspaceID: strings.TrimSpace(firstNonEmpty(c.Query("workspace_id"), c.Query("workspace"))),
		SessionID:   strings.TrimSpace(c.Query("session_id")),
		AgentName:   strings.TrimSpace(c.Query("agent_name")),
		Operator:    true,
	}, nil
}
