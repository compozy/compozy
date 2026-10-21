package core

import (
	"net/http"
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

// RequireProjectReference rejects User on surfaces that require a registered project.
func (h *BaseHandlers) RequireProjectReference(c *gin.Context, raw string) bool {
	if workspacepkg.ParseWorkspaceRef(raw).Kind != workspacepkg.RefUser {
		return true
	}
	err := workspacepkg.ErrUserScopeUnsupported
	if strings.Contains(c.FullPath(), "/worktrees") || c.Query("worktree") != "" || c.Query("worktree_id") != "" {
		err = workspacepkg.ErrUserScopeWorktreeUnsupported
	}
	h.respondError(c, http.StatusBadRequest, err)
	c.Abort()
	return false
}

// WorkspaceReferenceGuard rejects unsupported User selectors before registry lookup.
func (h *BaseHandlers) WorkspaceReferenceGuard(c *gin.Context) {
	routeRef := c.Param("workspace_id")
	if routeRef == "" && strings.Contains(c.FullPath(), "/workspaces/:id") {
		routeRef = c.Param("id")
	}
	for _, raw := range []string{routeRef, c.Query("workspace"), c.Query("workspace_id")} {
		if !h.RequireProjectReference(c, raw) {
			return
		}
	}
	c.Next()
}
