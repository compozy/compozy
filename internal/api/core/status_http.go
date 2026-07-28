package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetStatus returns the hard-cut runtime status payload shared by HTTP and UDS.
func (h *BaseHandlers) GetStatus(c *gin.Context) {
	payload, err := h.statusPayload(
		c.Request.Context(),
		firstNonEmptyString(c.Query("workspace_id"), c.Query("workspace")),
	)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}
