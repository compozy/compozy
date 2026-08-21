package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
)

// GetLoopRunNodes returns the computed roster for one Loop run.
func (h *BaseHandlers) GetLoopRunNodes(c *gin.Context) {
	readService, ok := h.requireLoopRunReadService(c)
	if !ok {
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

// GetLoopRunBriefing returns the server-owned verdict for one Loop run.
func (h *BaseHandlers) GetLoopRunBriefing(c *gin.Context) {
	readService, ok := h.requireLoopRunReadService(c)
	if !ok {
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

// GetLoopRunTimeline returns one durable timeline page for a Loop run.
func (h *BaseHandlers) GetLoopRunTimeline(c *gin.Context) {
	readService, ok := h.requireLoopRunReadService(c)
	if !ok {
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

func (h *BaseHandlers) requireLoopRunReadService(c *gin.Context) (LoopRunReadService, bool) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return nil, false
	}
	readService, ok := service.(LoopRunReadService)
	if !ok {
		h.respondLoopError(c, errors.New("loop run read service is unavailable"))
		return nil, false
	}
	return readService, true
}

func (h *BaseHandlers) respondLoopRunReadError(c *gin.Context, runID string, err error) {
	var invalidState *looppkg.InvalidNodeStateError
	var timelinePosition *looppkg.TimelinePositionError
	switch {
	case errors.Is(err, looppkg.ErrRunNotFound):
		c.JSON(http.StatusNotFound, contract.ErrorPayload{
			Error: "loop_run_not_found", Code: "loop_run_not_found", Details: map[string]string{"run_id": runID},
		})
	case errors.Is(err, looppkg.ErrTimelineBranchChanged):
		c.JSON(http.StatusConflict, contract.ErrorPayload{
			Error: "timeline_branch_changed",
			Code:  "timeline_branch_changed",
		})
	case errors.Is(err, looppkg.ErrInvalidTimelineCursor), errors.Is(err, looppkg.ErrInvalidRosterCursor):
		c.JSON(http.StatusBadRequest, contract.ErrorPayload{
			Error: "invalid_cursor",
			Code:  "invalid_cursor",
		})
	case errors.As(err, &timelinePosition):
		c.JSON(http.StatusBadRequest, contract.ErrorPayload{
			Error: timelinePosition.Error(),
			Code:  looppkg.ErrTimelinePositionBeyondHead.Error(),
			Details: map[string]string{
				"position": strconv.FormatInt(timelinePosition.Position, 10),
				"head_seq": strconv.FormatInt(timelinePosition.Head, 10),
			},
		})
	case errors.As(err, &invalidState):
		c.JSON(http.StatusBadRequest, contract.ErrorPayload{
			Error:   "invalid_node_state",
			Code:    "invalid_node_state",
			Details: map[string]string{"allowed": strings.Join(invalidState.Allowed, ",")},
		})
	default:
		h.respondLoopError(c, err)
	}
}
