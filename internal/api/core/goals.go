package core

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	sessionpkg "github.com/compozy/agh/internal/session"
	"github.com/gin-gonic/gin"
)

// GetSessionGoal returns the newest visible session-origin Goal or an explicit null projection.
func (h *BaseHandlers) GetSessionGoal(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	snapshot, err := service.GetSessionGoal(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("session_id"),
	)
	if err != nil {
		if errors.Is(err, sessionpkg.ErrSessionNotFound) {
			h.respondError(c, http.StatusNotFound, err)
		} else {
			h.respondLoopError(c, err)
		}
		return
	}
	payload, err := goalSnapshotPayload(snapshot)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionGoalResponse{Goal: payload})
}

// ListGoalTurns returns one validated run-wide Goal audit page.
func (h *BaseHandlers) ListGoalTurns(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	query, err := ParseGoalTurnListQuery(c)
	if err != nil {
		h.respondGoalTurnFilterError(c)
		return
	}
	page, err := service.ListGoalTurns(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("run_id"),
		query,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	payload, err := goalTurnPagePayload(page)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// ParseGoalTurnListQuery enforces the public cursor and filter contract without clamping.
func ParseGoalTurnListQuery(c *gin.Context) (GoalTurnListQuery, error) {
	if c == nil {
		return GoalTurnListQuery{}, errors.New("goal turn query context is required")
	}
	query := GoalTurnListQuery{NodeID: strings.TrimSpace(c.Query("node")), Limit: 50}
	if raw, present := c.GetQuery("node"); present && strings.TrimSpace(raw) == "" {
		return GoalTurnListQuery{}, errors.New("node must be nonempty when present")
	}
	if raw, present := c.GetQuery("item"); present {
		item, err := parseNonnegativeGoalQueryInt(raw, "item")
		if err != nil {
			return GoalTurnListQuery{}, err
		}
		if query.NodeID == "" {
			return GoalTurnListQuery{}, errors.New("item requires node")
		}
		query.ItemIndex = &item
	}
	if raw, present := c.GetQuery("after_seq"); present {
		after, err := parseNonnegativeGoalQueryInt64(raw, "after_seq")
		if err != nil {
			return GoalTurnListQuery{}, err
		}
		query.AfterSeq = after
	}
	if raw, present := c.GetQuery("limit"); present {
		limit, err := parseNonnegativeGoalQueryInt(raw, "limit")
		if err != nil || limit < 1 || limit > 200 {
			return GoalTurnListQuery{}, errors.New("limit must be between 1 and 200")
		}
		query.Limit = limit
	}
	return query, nil
}

func parseNonnegativeGoalQueryInt(raw string, name string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("%s must be nonempty", name)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a nonnegative integer", name)
	}
	return parsed, nil
}

func parseNonnegativeGoalQueryInt64(raw string, name string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("%s must be nonempty", name)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a nonnegative integer", name)
	}
	return parsed, nil
}

func (h *BaseHandlers) respondGoalTurnFilterError(c *gin.Context) {
	reason := contract.GoalReasonTurnFilterInvalid
	c.JSON(http.StatusUnprocessableEntity, contract.GoalCommandResult{
		Outcome: contract.GoalOutcomeError, ReasonCode: &reason,
	})
}

func goalContractValidationError(detail string) error {
	return fmt.Errorf("%w: %s", looppkg.ErrValidation, detail)
}
