package core

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetAgentActivity(c *gin.Context) {
	if !h.requireOperatorOrGrantedAgent(c, "agent activity") {
		return
	}
	days, err := parseAgentActivityDays(c.Query("days"))
	if err != nil {
		h.respondError(c, 422, err)
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if scope.AllProfiles {
		h.respondProfileReadScopeError(c, singleProfileReadError("agent activity belongs to one profile"))
		return
	}
	reader, err := h.agentActivityReader()
	if err != nil {
		h.respondError(c, 503, err)
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	selection := agentActivityScopes{handlers: h, context: c, profileID: scope.ProfileID, actor: actor}
	workspaces, err := selection.selectWorkspaces([]string{c.Query("workspace")})
	if err != nil {
		h.respondAgentActivityError(c, err)
		return
	}
	activity, err := reader.ReadActivity(c.Request.Context(), agentinstance.ActivityQuery{
		ProfileID: scope.ProfileID, WorkspaceIDs: workspaces, AgentName: c.Param("name"),
		Since: store.LocalDayStart(now, days-1),
	})
	if err != nil {
		h.respondError(c, 500, err)
		return
	}
	result := activity
	for i := range result.CostByProject {
		result.CostByProject[i].Scope = selection.labels[result.CostByProject[i].WorkspaceID]
	}
	sortAgentActivity(&result)
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) resolveAgentActivityWorkspace(ctx context.Context, ref string) (string, bool, error) {
	if ref == "" || ref == userScopeWorkspaceRef {
		return "", ref != "", nil
	}
	resolved, err := h.Workspaces.Resolve(ctx, ref)
	if err != nil {
		return "", true, err
	}
	return resolved.ID, true, nil
}

func (h *BaseHandlers) agentActivityWorkspaceLabel(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "Global", nil
	}
	resolved, err := h.Workspaces.Resolve(ctx, id)
	if err != nil {
		return "", err
	}
	return resolved.Name, nil
}

func (h *BaseHandlers) authorizeAgentActivityWorkspace(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	profileID, workspaceID string,
) (bool, error) {
	if actor.Operator {
		return true, nil
	}
	if h.WorkspaceAccess == nil {
		return false, nil
	}
	decision, err := h.WorkspaceAccess.Authorize(ctx, workspaceaccess.Request{
		Actor:    actor,
		Target:   workspaceaccess.ScopeTarget(workspaceID, profileID),
		ReadOnly: true,
		Seam:     workspaceaccess.SeamTool,
	})
	return decision.Allowed, err
}

func parseAgentActivityDays(raw string) (int, error) {
	if raw == "" {
		return 30, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 1 || days > 365 {
		return 0, errors.New("days must be from 1 to 365")
	}
	return days, nil
}

func (h *BaseHandlers) agentActivityReader() (agentinstance.ActivityReader, error) {
	if h.AgentInstances == nil {
		return nil, errors.New("agent activity unavailable")
	}
	reader, ok := h.AgentInstances.Store.(agentinstance.ActivityReader)
	if !ok {
		return nil, errors.New("agent activity unavailable")
	}
	return reader, nil
}

func sortAgentActivity(result *agentinstance.Activity) {
	slices.SortFunc(result.ReportsByDay, func(a, b agentinstance.ActivityDay) int { return cmp.Compare(a.Day, b.Day) })
	slices.SortFunc(
		result.CostByProject,
		func(a, b agentinstance.ActivityCost) int { return cmp.Compare(a.WorkspaceID, b.WorkspaceID) },
	)
	slices.SortFunc(result.BusyByHour, func(a, b agentinstance.ActivityHour) int {
		if a.Weekday != b.Weekday {
			return cmp.Compare(a.Weekday, b.Weekday)
		}
		return cmp.Compare(a.Hour, b.Hour)
	})
}
