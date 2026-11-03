package core

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

// TerminalProvider is the daemon-owned terminal domain authority.
type TerminalProvider interface{ terminalpkg.Manager }

// terminalReadService resolves a terminal read that admits the owner-labeled all-profiles
// aggregate through the shared profile read policy and resolves one profile historically.
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
	resolved, err := h.resolveTerminalProfileReadSelection(c, workspaceID)
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
		resolved, err = h.resolveTerminalProfileReadSelection(c, workspaceID)
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
// terminal in whichever profile owns it; the payload's profile_name labels that owner. A
// terminal the aggregate list no longer shows answers like a single-profile read, so an
// expired terminal still reports terminal_expired.
func (h *BaseHandlers) terminalInScope(
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
	notFound := &terminalpkg.Error{
		Code: terminalpkg.ErrorCodeNotFound, Message: "terminal not found", Err: terminalpkg.ErrNotFound,
	}
	profileIDs, err := h.terminalOwnerProfileIDs(ctx)
	if err != nil {
		return nil, notFound
	}
	for _, profileID := range profileIDs {
		if _, err := service.Get(ctx, workspaceID, profileID, id); errors.Is(err, terminalpkg.ErrExpired) {
			return nil, err
		}
	}
	return nil, notFound
}

// terminalOwnerProfileIDs lists the profiles an aggregate terminal read resolves one owner at a
// time, in a stable order.
func (h *BaseHandlers) terminalOwnerProfileIDs(ctx context.Context) ([]string, error) {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(owners))
	for _, owner := range owners {
		if owner.ID != "" && !slices.Contains(ids, owner.ID) {
			ids = append(ids, owner.ID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}
