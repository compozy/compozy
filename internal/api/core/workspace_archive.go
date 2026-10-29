package core

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) ArchiveWorkspace(c *gin.Context) {
	h.setWorkspaceArchived(c, true)
}

func (h *BaseHandlers) UnarchiveWorkspace(c *gin.Context) {
	h.setWorkspaceArchived(c, false)
}

func (h *BaseHandlers) setWorkspaceArchived(c *gin.Context, archived bool) {
	ws, err := h.Workspaces.SetArchived(c.Request.Context(), workspaceRefFromRoute(c), archived)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.WorkspaceResponse{Workspace: WorkspacePayloadFromWorkspace(ws)})
}
