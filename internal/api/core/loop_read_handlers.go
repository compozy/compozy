package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func (h *BaseHandlers) GetLoopRunNodes(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	readService, ok := service.(LoopRunReadService)
	if !ok {
		h.respondLoopError(c, errors.New("loop run read service is unavailable"))
		return
	}
	generation, err := ParseOptionalInt(c.Query("generation"))
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: generation query: %v", looppkg.ErrValidation, err))
		return
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: limit query: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := readService.GetLoopRunNodes(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("run_id"),
		looppkg.RosterQuery{
			State:      looppkg.NodeStateFilter(strings.TrimSpace(c.Query("state"))),
			Generation: generation,
			Cursor:     strings.TrimSpace(c.Query("cursor")),
			Limit:      limit,
		},
	)
	if err != nil {
		h.respondLoopRunReadError(c, c.Param("run_id"), err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) GetLoopRunBriefing(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	readService, ok := service.(LoopRunReadService)
	if !ok {
		h.respondLoopError(c, errors.New("loop run read service is unavailable"))
		return
	}
	response, err := readService.GetLoopRunBriefing(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("run_id"),
	)
	if err != nil {
		h.respondLoopRunReadError(c, c.Param("run_id"), err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) GetLoopRunTimeline(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	readService, ok := service.(LoopRunReadService)
	if !ok {
		h.respondLoopError(c, errors.New("loop run read service is unavailable"))
		return
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: limit query: %v", looppkg.ErrValidation, err))
		return
	}
	afterSequence, err := ParseOptionalInt64(c.Query("after_sequence"))
	if err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: after_sequence query: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := readService.GetLoopRunTimeline(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("run_id"),
		looppkg.TimelineQuery{
			View:     looppkg.TimelineView(strings.TrimSpace(c.Query("view"))),
			Cursor:   strings.TrimSpace(c.Query("cursor")),
			Limit:    limit,
			AfterSeq: afterSequence,
		},
	)
	if err != nil {
		h.respondLoopRunReadError(c, c.Param("run_id"), err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) respondLoopRunReadError(c *gin.Context, runID string, err error) {
	var invalidState *looppkg.InvalidNodeStateError
	switch {
	case errors.Is(err, looppkg.ErrRunNotFound):
		c.JSON(http.StatusNotFound, gin.H{bridgesErrorKey: "loop_run_not_found", "run_id": runID})
	case errors.Is(err, looppkg.ErrTimelineBranchChanged):
		c.JSON(http.StatusConflict, gin.H{bridgesErrorKey: "timeline_branch_changed"})
	case errors.Is(err, looppkg.ErrInvalidTimelineCursor), errors.Is(err, looppkg.ErrInvalidRosterCursor):
		c.JSON(http.StatusBadRequest, gin.H{bridgesErrorKey: "invalid_cursor"})
	case errors.Is(err, looppkg.ErrTimelinePositionBeyondHead):
		c.JSON(http.StatusBadRequest, gin.H{bridgesErrorKey: err.Error()})
	case errors.As(err, &invalidState):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_node_state",
			"allowed": invalidState.Allowed,
		})
	default:
		h.respondLoopError(c, err)
	}
}
