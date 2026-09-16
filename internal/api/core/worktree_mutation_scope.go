package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) worktreeMutationTarget(c *gin.Context, workspaceID, ref string) (string, bool) {
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return "", false
	}
	item, err := h.Worktrees.Resolve(c.Request.Context(), workspaceID, ref)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return "", false
	}
	if item == nil || !scope.Matches(item.ProfileID) {
		h.respondError(c, http.StatusNotFound, worktree.ErrNotFound)
		return "", false
	}
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return "", false
	}
	owner, exists := owners[item.ProfileID]
	if !exists || owner.Archived {
		h.respondError(c, http.StatusForbidden, worktree.ErrNotReady)
		return "", false
	}
	return item.ID, true
}
