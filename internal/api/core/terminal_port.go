package core

import (
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

// TerminalProvider is the daemon-owned terminal domain authority.
type TerminalProvider interface{ terminalpkg.Manager }

func (h *BaseHandlers) terminalAggregateService(
	c *gin.Context,
) (terminalpkg.Manager, store.ReadScope, bool) {
	if h == nil || h.Terminal == nil {
		if h != nil {
			h.respondTerminalUnavailable(c)
		}
		return nil, store.ReadScope{}, false
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	resolved, err := h.resolveTerminalProfileReadSelection(c, workspaceID)
	if err != nil {
		h.respondTerminalProfileError(c, err)
		return nil, store.ReadScope{}, false
	}
	return h.Terminal, resolved.Scope, true
}

func (h *BaseHandlers) terminalService(
	c *gin.Context,
	mutation bool,
) (terminalpkg.Manager, string, bool) {
	if h == nil || h.Terminal == nil {
		if h != nil {
			h.respondTerminalUnavailable(c)
		}
		return nil, "", false
	}
	var scopeID string
	var err error
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	if mutation {
		resolved, scopeErr := h.resolveProfileMutationSelectionForWorkspace(c, workspaceID)
		err, scopeID = scopeErr, resolved.Scope.ProfileID
	} else {
		resolved, scopeErr := h.resolveTerminalProfileReadSelection(c, workspaceID)
		err, scopeID = scopeErr, resolved.Scope.ProfileID
		if scopeErr == nil && resolved.Scope.AllProfiles {
			err = &profilepkg.Error{
				Code: profileSelectionConflictCode, Message: "this terminal read belongs to one profile",
				Action: "choose one profile", Cause: profilepkg.ErrInvalidInput,
			}
		}
	}
	if err != nil {
		h.respondTerminalProfileError(c, err)
		return nil, "", false
	}
	return h.Terminal, scopeID, true
}
