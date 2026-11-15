package udsapi

import (
	"net/http"

	core "github.com/compozy/compozy/internal/api/core"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/gin-gonic/gin"
)

func (h *Handlers) reportHostedMCPServed(c *gin.Context) {
	if h.HostedMCP == nil {
		core.RespondError(c, http.StatusServiceUnavailable, mcppkg.ErrHostedDisabled, false)
		return
	}
	var req mcppkg.HostedServedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	peer, err := mcppkg.PeerInfoFromContext(c.Request.Context())
	if err != nil {
		h.respondHostedMCPPeerError(c, err)
		return
	}
	if err := h.HostedMCP.ReportServedCatalog(c.Request.Context(), req, peer); err != nil {
		respondHostedMCPError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
