package core

import (
	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

const networkWorkspaceAction = "network.workspace"

// AuthorizeNetworkWorkspaceAccess enforces agent workspace policy on every workspace Network route.
func (h *BaseHandlers) AuthorizeNetworkWorkspaceAccess(c *gin.Context) {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		c.Next()
		return
	}
	targetWorkspaceID, ok := h.requireRouteWorkspaceID(c)
	if !ok {
		c.Abort()
		return
	}
	caller, err := h.resolveAgentCaller(c.Request.Context(), credentials, networkWorkspaceAction)
	if err == nil {
		req := workspaceAccessRequestForCaller(caller, targetWorkspaceID)
		req.Seam = workspaceaccess.SeamCoordination
		err = agentidentity.ValidateWorkspaceAccess(c.Request.Context(), h.WorkspaceAccess, req)
	}
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		c.Abort()
		return
	}
	c.Next()
}
