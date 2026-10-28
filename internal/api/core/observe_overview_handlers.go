package core

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/observe"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

var errOverviewUsageWindow = errors.New("usage_window must be 7, 30, or 90")

// ObserveOverview returns the observer-backed home overview read model.
func (h *BaseHandlers) ObserveOverview(c *gin.Context) {
	if !h.requireOperatorOrGrantedAgent(c, "overview") {
		return
	}
	observer, ok := h.requireTaskObserver(c)
	if !ok {
		return
	}

	query, ok := h.attentionOverviewQuery(c)
	if !ok {
		return
	}

	usageWindow, err := parseOverviewUsageWindow(c.Query("usage_window"))
	if err != nil {
		h.respondError(c, http.StatusUnprocessableEntity, err)
		return
	}

	query.UsageWindowDays = usageWindow

	view, err := observer.QueryObserveOverview(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	if query.TaskScope == taskpkg.CatalogScopeAll && h.Logger != nil {
		h.Logger.DebugContext(
			c.Request.Context(),
			"overview aggregate read",
			"all_workspaces",
			true,
			"result_count",
			view.Attention.Total,
		)
	}
	c.JSON(http.StatusOK, contract.ObserveOverviewResponse{Overview: ObserveOverviewPayloadFromView(&view)})
}

func parseOverviewUsageWindow(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	window, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, NewTaskValidationError(err)
	}
	if err := observe.ValidateUsageWindowDays(window); err != nil {
		return 0, NewTaskValidationError(errOverviewUsageWindow)
	}
	return window, nil
}

func (h *BaseHandlers) resolveOverviewWorkspace(c *gin.Context, query *observe.OverviewQuery) bool {
	query.TaskScope = taskpkg.CatalogScopeGlobal
	breadth, err := parseWorkspaceBreadth(
		c,
		c.Param("workspace_id"),
		breadthEncoding{Selector: workspaceQueryKey, OmissionIsAll: true},
		h.Workspaces,
	)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return false
	}
	if len(c.Request.URL.Query()["workspace_ids"]) != 0 {
		h.respondError(c, http.StatusBadRequest, invalidWorkspaceBreadth("overview does not support workspace_ids"))
		return false
	}
	all, err := ParseOptionalBool(c.Query("all_workspaces"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return false
	}
	if all {
		if !h.requireOperatorOrGrantedAgent(c, "all-workspaces overview") {
			return false
		}
		query.TaskScope = taskpkg.CatalogScopeAll
		query.WorkspaceIDs = breadth.WorkspaceIDs
		return true
	}
	if breadth.WorkspaceID != "" {
		scope := taskpkg.CatalogScopeWorkspace
		if err := h.resolveTaskCatalogWorkspace(
			c.Request.Context(),
			breadth.WorkspaceID,
			&scope,
			&query.WorkspaceID,
		); err != nil {
			h.respondError(c, StatusForTaskError(err), err)
			return false
		}
		query.TaskScope = scope
	}
	return true
}
