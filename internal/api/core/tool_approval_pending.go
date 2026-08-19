package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetPendingToolApproval(c *gin.Context) {
	if h.ApprovalCoordinator == nil {
		h.respondPendingToolApprovalError(c, errors.New("tool approval coordinator is unavailable"))
		return
	}
	status, err := h.ApprovalCoordinator.Status(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.respondPendingToolApprovalError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolApprovalStatusFromDomain(status))
}

func (h *BaseHandlers) CancelPendingToolApproval(c *gin.Context) {
	if h.ApprovalCoordinator == nil {
		h.respondPendingToolApprovalError(c, errors.New("tool approval coordinator is unavailable"))
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if err := h.ApprovalCoordinator.Cancel(c.Request.Context(), id); err != nil {
		h.respondPendingToolApprovalError(c, err)
		return
	}
	status, err := h.ApprovalCoordinator.Status(c.Request.Context(), id)
	if err != nil {
		h.respondPendingToolApprovalError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.ToolApprovalStatusFromDomain(status))
}

func (h *BaseHandlers) respondPendingToolApprovalError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, toolspkg.ErrApprovalNotFound):
		c.JSON(http.StatusNotFound, contract.CmdPaletteError{
			Error: "approval_not_found", Message: "approval not found",
		})
	case errors.Is(err, toolspkg.ErrApprovalTerminal):
		c.JSON(http.StatusConflict, contract.CmdPaletteError{
			Error: "approval_terminal", Message: "approval is already terminal",
		})
	default:
		if h.Logger != nil {
			h.Logger.Error("pending tool approval request failed", "error", err)
		}
		c.JSON(http.StatusServiceUnavailable, contract.CmdPaletteError{
			Error: "runtime_unavailable", Message: "tool approval runtime is unavailable",
		})
	}
}
