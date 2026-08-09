package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

// GetStatus returns the hard-cut runtime status payload shared by HTTP and UDS.
func (h *BaseHandlers) GetStatus(c *gin.Context) {
	ctx, err := h.gatewayStatusReadContext(c)
	if err != nil {
		h.respondGatewayError(c, errors.Join(gateway.ErrIngressForbidden, err))
		return
	}
	payload, err := h.statusPayload(
		ctx,
		firstNonEmptyString(c.Query("workspace_id"), c.Query("workspace")),
	)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}
