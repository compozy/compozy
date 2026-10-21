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
	if all, _ := ParseOptionalBool(c.Query("all_workspaces")); all && !h.requireAggregateSurface(c) {
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
	query, err := ParseAutomationRunQuery(c)
	if err != nil {
		if _, ok := err.(*workspaceBreadthError); ok {
			h.respondAggregateBreadthError(c, err)
		} else {
			h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		}
		return
	}
	query.ReadScope = readScope

	paged := query.AllWorkspaces || query.Cursor != "" || query.WorkspaceID != "" || len(query.WorkspaceIDs) > 0
	limit := query.Limit
	if limit == 0 && paged {
		limit = automationpkg.DefaultListLimit
	}
	if limit < 0 || (paged && limit > automationpkg.MaxListLimit) {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(fmt.Errorf("invalid run limit")))
		return
	}
	if paged && limit > 0 {
		query.Limit = limit + 1
	}
	runs, err := manager.ListRuns(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	var page *contract.CursorPagePayload
	if paged && limit > 0 {
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
