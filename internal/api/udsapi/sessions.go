package udsapi

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
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
	result, err := h.Sessions.ApprovePermission(c.Request.Context(), sessionID, approve)
	if err != nil {
		core.RespondError(c, core.StatusForSessionError(err), err, false)
		return
	}

	c.JSON(http.StatusOK, contract.SessionApprovalResponse{
		Outcome:          result.Outcome,
		InteractionID:    result.InteractionID,
		RequestID:        result.RequestID,
		Decision:         result.Decision,
		ResolvedDecision: result.ResolvedDecision,
	})
}
