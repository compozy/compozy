package httpapi

import (
	"errors"
	"net/http"

	"github.com/compozy/agh/internal/api/core"
	"github.com/gin-gonic/gin"
)

var errLoopbackMCPCallbackRequired = errors.New(
	"automatic MCP OAuth callback is disabled unless the daemon is bound to a loopback host; use manual exchange",
)

func (h *Handlers) completeMCPAuthCallback(c *gin.Context) {
	if !isLoopbackHost(canonicalHost(h.boundHost)) {
		core.RespondError(c, http.StatusForbidden, errLoopbackMCPCallbackRequired, false)
		return
	}
	h.CompleteSettingsMCPAuthCallback(c)
}
