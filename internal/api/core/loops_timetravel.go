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

const loopActionTimeTravel = "loops.timetravel"

func (h *BaseHandlers) DiffLoopRun(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, false) {
		return
	}
	query := looppkg.DiffQuery{AgainstRunID: looppkg.RunID(strings.TrimSpace(c.Query("against_run")))}
	var err error
	query.Generation, err = parseLoopGenerationQuery(c.Query("generation"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	query.AgainstGeneration, err = parseLoopGenerationQuery(c.Query("against_generation"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	response, err := service.DiffLoopRun(c.Request.Context(), c.Param("workspace_id"), c.Param("run_id"), query)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) RerunLoopRun(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, true) {
		return
	}
	var request contract.RerunLoopRequest
	if err := decodeStrictLoopJSONBody(c, &request); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode Loop rerun: %v", looppkg.ErrValidation, err))
		return
	}
	workspaceID := c.Param("workspace_id")
	actor, err := h.taskActorContextForWorkspace(c, loopActionTimeTravel, workspaceID)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	response, err := service.RerunLoopRun(
		c.Request.Context(), workspaceID, c.Param("run_id"), request, actor,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) ForkLoopRun(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	if !h.requireLoopRunProfile(c, service, true) {
		return
	}
	var request contract.ForkLoopRequest
	if err := decodeStrictLoopJSONBody(c, &request); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode Loop fork: %v", looppkg.ErrValidation, err))
		return
	}
	workspaceID := c.Param("workspace_id")
	actor, err := h.taskActorContextForWorkspace(c, loopActionTimeTravel, workspaceID)
	if err != nil {
		h.respondError(c, StatusForTaskError(err), err)
		return
	}
	response, err := service.ForkLoopRun(
		c.Request.Context(), workspaceID, c.Param("run_id"), request, actor,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusCreated, response)
}

func parseLoopGenerationQuery(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || generation < 1 {
		return 0, fmt.Errorf("%w: generation must be positive", looppkg.ErrValidation)
	}
	return generation, nil
}
