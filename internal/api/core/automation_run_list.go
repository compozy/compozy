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
	manager, ok := h.requireAutomationManager(c)
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
	limit, paged, err := applyAutomationRunPageLimit(&query)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	runs, err := manager.ListRuns(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	var page *contract.CursorPagePayload
	if paged {
		page = &contract.CursorPagePayload{Limit: limit, HasMore: len(runs) > limit}
	}
	if page != nil && page.HasMore {
		runs = runs[:limit]
		page.NextCursor, err = modelpkg.EncodeRunCursor(query, runs[len(runs)-1])
		if err != nil {
			h.respondError(c, http.StatusInternalServerError, err)
			return
		}
	}
	runPayloads := RunPayloadsFromRuns(runs)
	if err := h.decorateAutomationRunOwners(c.Request.Context(), runPayloads); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.RunsResponse{Runs: runPayloads, Page: page})
}

// applyAutomationRunPageLimit pages bounded reads (aggregate, cursor, or project filter) and asks
// the store for one look-ahead row; a plain unbounded history read stays unpaged.
func applyAutomationRunPageLimit(query *automationpkg.RunQuery) (int, bool, error) {
	paged := query.AllWorkspaces || query.Cursor != "" || query.WorkspaceID != "" || len(query.WorkspaceIDs) > 0
	limit := query.Limit
	if limit == 0 && paged {
		limit = automationpkg.DefaultListLimit
	}
	if limit < 0 || (paged && limit > automationpkg.MaxListLimit) {
		return 0, false, NewAutomationValidationError(fmt.Errorf("invalid run limit"))
	}
	if !paged || limit == 0 {
		return limit, false, nil
	}
	query.Limit = limit + 1
	return limit, true, nil
}
