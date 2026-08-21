package core

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) AmendLoopNode(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, true) {
		return
	}
	var req contract.LoopNodeAmendRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode Loop amendment: %v", looppkg.ErrValidation, err))
		return
	}
	workspaceID := c.Param("workspace_id")
	actor, err := h.taskActorContextForWorkspace(c, loopActionRespond, workspaceID)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	response, err := service.AmendLoopNode(c.Request.Context(), workspaceID,
		c.Param("run_id"), c.Param("node_id"), req, actor)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}
