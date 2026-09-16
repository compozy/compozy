package core

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) worktreeMutationTarget(c *gin.Context, workspaceID, ref string) (string, bool) {
	item, err := h.Worktrees.Resolve(c.Request.Context(), workspaceID, ref)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return "", false
	}
	if item == nil {
		h.respondError(c, http.StatusNotFound, worktree.ErrNotFound)
		return "", false
	}
	if !h.requireWorktreeMutationProfile(c, item) {
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

// Operator requests predating the optional profile selector already identify their
// owner through the target record. Translate that shape at this boundary; agent
// credentials and explicit selectors always retain their profile authority.
func (h *BaseHandlers) requireWorktreeMutationProfile(c *gin.Context, item *worktree.Worktree) bool {
	if strings.TrimSpace(c.Query("profile")) == "" && c.Query("all_profiles") == "" &&
		!hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return true
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return false
	}
	if !scope.Matches(item.ProfileID) {
		h.respondError(c, http.StatusNotFound, worktree.ErrNotFound)
		return false
	}
	return true
}
