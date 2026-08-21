package core

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

const loopActionRespond = "loops.respond"

func (h *BaseHandlers) ListLoopRequests(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if runID := strings.TrimSpace(c.Query("run_id")); runID != "" {
		if !h.requireLoopRunProfileByID(c, service, runID, false) {
			return
		}
	}
	query := LoopRequestListQuery{RunID: c.Query("run_id"), State: c.Query("state"), Cursor: c.Query("cursor")}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			h.respondLoopError(c, fmt.Errorf("%w: request limit is invalid", looppkg.ErrValidation))
			return
		}
		query.Limit = limit
	}
	response, err := service.ListLoopRequests(c.Request.Context(), c.Param("workspace_id"), query)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) GetLoopRequest(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, false) {
		return
	}
	itemIndex, err := optionalRequestItemIndex(c)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	generation, err := requiredRequestGeneration(c)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	response, err := service.GetLoopRequest(c.Request.Context(), c.Param("workspace_id"),
		c.Param("run_id"), generation, c.Param("node_id"), itemIndex)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func requiredRequestGeneration(c *gin.Context) (int, error) {
	raw := c.Query("generation")
	generation, err := strconv.Atoi(raw)
	if err != nil || generation < 1 {
		return 0, fmt.Errorf("%w: request generation is invalid", looppkg.ErrValidation)
	}
	return generation, nil
}

func (h *BaseHandlers) RespondLoopRequest(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, true) {
		return
	}
	var req contract.RespondLoopRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode Loop response: %v", looppkg.ErrValidation, err))
		return
	}
	workspaceID := c.Param("workspace_id")
	actor, err := h.taskActorContextForWorkspace(c, loopActionRespond, workspaceID)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	response, err := service.RespondLoopRequest(c.Request.Context(), workspaceID,
		c.Param("run_id"), c.Param("node_id"), req, actor)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func optionalRequestItemIndex(c *gin.Context) (int, error) {
	raw := c.Query("item_index")
	if raw == "" {
		return 0, nil
	}
	itemIndex, err := strconv.Atoi(raw)
	if err != nil || itemIndex < 0 {
		return 0, fmt.Errorf("%w: request item_index is invalid", looppkg.ErrValidation)
	}
	return itemIndex, nil
}
