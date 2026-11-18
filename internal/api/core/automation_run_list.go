package core

import (
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	modelpkg "github.com/compozy/compozy/internal/automation/model"
	"github.com/gin-gonic/gin"
)

// ListAutomationRuns returns filtered automation run history.
func (h *BaseHandlers) ListAutomationRuns(c *gin.Context) {
	if all, err := ParseOptionalBool(c.Query("all_workspaces")); err == nil && all && !h.requireAggregateSurface(c) {
		return
	}
	_, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}

	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	query, err := ParseAutomationRunQuery(c, h.Workspaces)
	if err != nil {
		if _, ok := err.(*workspaceBreadthError); ok {
			h.respondAggregateBreadthError(c, err)
		} else {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		}
		return
	}
	query.ReadScope = readScope
	h.respondAutomationRuns(c, query)
}

func (h *BaseHandlers) respondAutomationRuns(c *gin.Context, query automationpkg.RunQuery) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	limit, err := PrepareAutomationRunPage(&query)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	runs, err := manager.ListRuns(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	response, err := AutomationRunPage(query, runs, limit)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	runPayloads := response.Runs
	if err := h.decorateAutomationRunOwners(c.Request.Context(), runPayloads); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PrepareAutomationRunPage bounds public history reads and reserves one look-ahead row.
func PrepareAutomationRunPage(query *automationpkg.RunQuery) (int, error) {
	limit := query.Limit
	if limit == 0 {
		limit = automationpkg.DefaultListLimit
	}
	if limit < 1 || limit > automationpkg.MaxListLimit {
		return 0, NewAutomationValidationError(
			fmt.Errorf("run limit must be between 0 and %d", automationpkg.MaxListLimit),
		)
	}
	query.Limit = limit + 1
	return limit, nil
}

func AutomationRunPage(
	query automationpkg.RunQuery,
	runs []automationpkg.Run,
	limit int,
) (contract.RunsResponse, error) {
	page := &contract.CursorPagePayload{Limit: limit, HasMore: len(runs) > limit}
	if page.HasMore {
		runs = runs[:limit]
		cursor, err := modelpkg.EncodeRunCursor(query, runs[len(runs)-1])
		if err != nil {
			return contract.RunsResponse{}, err
		}
		page.NextCursor = cursor
	}
	return contract.RunsResponse{Runs: RunPayloadsFromRuns(runs), Page: page}, nil
}
