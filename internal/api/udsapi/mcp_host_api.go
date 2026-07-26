package udsapi

import (
	"net/http"

	"github.com/compozy/agh/internal/api/core"
	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/gin-gonic/gin"
)

func registerMCPHostAPIRoutes(api gin.IRouter, handlers *Handlers) {
	api.POST("/internal/mcp/host-api/invoke", handlers.invokeMCPHostAPI)
	api.POST("/internal/mcp/host-api/session/close", handlers.closeMCPHostAPISession)
}

func registerMCPRoutes(api gin.IRouter, handlers *Handlers) {
	registerHostedMCPRoutes(api, handlers)
	registerMCPHostAPIRoutes(api, handlers)
}

func (h *Handlers) invokeMCPHostAPI(c *gin.Context) {
	if h == nil || h.MCPHostAPI == nil {
		core.RespondError(c, http.StatusServiceUnavailable, errMCPHostAPIUnavailable, false)
		return
	}
	var request mcppkg.HostAPIInvokeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	result, err := h.MCPHostAPI.InvokeHostAPI(
		c.Request.Context(),
		request.ServeSessionID,
		request.Workspace,
		request.Method,
		request.Params,
	)
	if err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, false)
		return
	}
	c.JSON(http.StatusOK, mcppkg.HostAPIInvokeResponse{Result: result})
}

func (h *Handlers) closeMCPHostAPISession(c *gin.Context) {
	if h == nil || h.MCPHostAPI == nil {
		core.RespondError(c, http.StatusServiceUnavailable, errMCPHostAPIUnavailable, false)
		return
	}
	var request mcppkg.HostAPISessionCloseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, false)
		return
	}
	if err := h.MCPHostAPI.CloseHostAPISession(c.Request.Context(), request.ServeSessionID); err != nil {
		core.RespondError(c, http.StatusInternalServerError, err, false)
		return
	}
	c.Status(http.StatusNoContent)
}
