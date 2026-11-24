package core

import (
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/remotehost"
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
	scope := toolspkg.Scope{
		ProfileID:   strings.TrimSpace(selection.Scope.ProfileID),
		WorkspaceID: strings.TrimSpace(firstNonEmpty(c.Query("workspace_id"), c.Query("workspace"))),
		Operator:    true,
	}
	// A remote session authenticates the device, never a local agent session.
	if _, remote := remotehost.PrincipalFromContext(c.Request.Context()); !remote {
		scope.SessionID = strings.TrimSpace(c.Query("session_id"))
		scope.AgentName = strings.TrimSpace(c.Query("agent_name"))
	}
	return scope, nil
}
