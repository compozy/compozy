package udsapi

import (
	"fmt"
	"net/http"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/gin-gonic/gin"
)

func (h *Handlers) approveSession(c *gin.Context) {
	var req contract.ApproveSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.RespondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("udsapi: decode approve session request: %w", err),
			false,
		)
		return
	}

	approve := acp.ApproveRequest{
		RequestID: req.RequestID,
		TurnID:    req.TurnID,
		Decision:  req.Decision,
	}
	if err := approve.Validate(); err != nil {
		core.RespondError(c, http.StatusBadRequest, err, false)
		return
	}

	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	if err := h.Sessions.ApprovePermission(c.Request.Context(), sessionID, approve); err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}

	c.JSON(http.StatusOK, contract.SessionApprovalResponse{Status: "approved"})
}

func (h *Handlers) cancelSessionPrompt(c *gin.Context) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	if err := h.Sessions.CancelPrompt(c.Request.Context(), sessionID); err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}

	c.Status(http.StatusOK)
}
