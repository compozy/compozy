package core

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

// TerminalProvider is the daemon-owned terminal domain authority.
type TerminalProvider interface{ terminalpkg.Manager }

// terminalReadService resolves a terminal read that admits the owner-labeled all-profiles
// aggregate, through the same profile read policy every other read uses.
func (h *BaseHandlers) terminalReadService(
	c *gin.Context,
) (terminalpkg.Manager, store.ReadScope, bool) {
	if h == nil || h.Terminal == nil {
		if h != nil {
			h.respondTerminalUnavailable(c)
		}
		return nil, store.ReadScope{}, false
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	resolved, err := h.resolveProfileReadSelectionForWorkspace(c, workspaceID)
	if err != nil {
		h.respondTerminalProfileError(c, err)
		return nil, store.ReadScope{}, false
	}
	return h.Terminal, resolved.Scope, true
}

// terminalService resolves the one profile that owns a terminal interaction or mutation.
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
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	var resolved resolvedProfileReadScope
	var err error
	if mutation {
		resolved, err = h.resolveProfileMutationSelectionForWorkspace(c, workspaceID)
	} else {
		resolved, err = h.resolveProfileReadSelectionForWorkspace(c, workspaceID)
		if err == nil && resolved.Scope.AllProfiles {
			err = singleProfileReadError("this terminal interaction belongs to one profile")
		}
	}
	if err != nil {
		h.respondTerminalProfileError(c, err)
		return nil, "", false
	}
	return h.Terminal, resolved.Scope.ProfileID, true
}

// terminalInScope reads one terminal from the resolved scope. The aggregate finds the
// terminal in whichever profile owns it; the payload's profile_name labels that owner.
func terminalInScope(
	ctx context.Context,
	service terminalpkg.Manager,
	workspaceID string,
	scope store.ReadScope,
	id terminalpkg.ID,
) (*terminalpkg.Info, error) {
	if !scope.AllProfiles {
		return service.Get(ctx, workspaceID, scope.ProfileID, id)
	}
	items, err := service.List(ctx, workspaceID, scope)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].ID == id {
			return &items[index], nil
		}
	}
	return nil, &terminalpkg.Error{
		Code: terminalpkg.ErrorCodeNotFound, Message: "terminal not found", Err: terminalpkg.ErrNotFound,
	}
}
