package core

import (
	"cmp"
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
	days := 30
	if raw := c.Query("days"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 365 {
			h.respondError(c, 422, errors.New("days must be from 1 to 365"))
			return
		}
		days = value
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if scope.AllProfiles {
		h.respondError(c, 422, errors.New("choose one profile"))
		return
	}
	if h.AgentInstances == nil {
		h.respondError(c, 503, errors.New("agent activity unavailable"))
		return
	}
	reader, ok := h.AgentInstances.Store.(agentinstance.ActivityReader)
	if !ok {
		h.respondError(c, 503, errors.New("agent activity unavailable"))
		return
	}
	allowed, err := h.aggregateWorkspaceSet(c, WorkspaceBreadth{AllWorkspaces: true})
	if err != nil {
		h.respondError(c, 500, err)
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
	result := agentinstance.Activity{
		ReportsByDay:  []agentinstance.ActivityDay{},
		CostByProject: []agentinstance.ActivityCost{},
		BusyByHour:    []agentinstance.ActivityHour{},
	}
	for workspace := range allowed {
		if !actor.Operator {
			if h.WorkspaceAccess == nil {
				continue
			}
			decision, err := h.WorkspaceAccess.Authorize(
				c.Request.Context(),
				workspaceaccess.Request{
					Actor:    actor,
					Target:   workspaceaccess.ScopeTarget(workspace, scope.ProfileID),
					ReadOnly: true,
					Seam:     workspaceaccess.SeamTool,
				},
			)
			if err != nil {
				h.respondError(c, 500, err)
				return
			}
			if !decision.Allowed {
				continue
			}
		}
		activity, err := reader.ReadActivity(
			c.Request.Context(),
			agentinstance.ActivityQuery{
				ProfileID:   scope.ProfileID,
				WorkspaceID: workspace,
				AgentName:   c.Param("name"),
				Since:       store.LocalDayStart(now, days-1),
			},
		)
		if err != nil {
			h.respondError(c, 500, err)
			return
		}
		label := "Global"
		if workspace != "" {
			resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspace)
			if err != nil {
				h.respondError(c, 500, err)
				return
			}
			label = resolved.Name
		}
		for _, cost := range activity.CostByProject {
			cost.Scope = label
			result.CostByProject = append(result.CostByProject, cost)
		}
		for _, day := range activity.ReportsByDay {
			index := slices.IndexFunc(
				result.ReportsByDay,
				func(item agentinstance.ActivityDay) bool { return item.Day == day.Day },
			)
			if index < 0 {
				result.ReportsByDay = append(result.ReportsByDay, day)
			} else {
				result.ReportsByDay[index].Quiet += day.Quiet
				result.ReportsByDay[index].Told += day.Told
				result.ReportsByDay[index].NeededYou += day.NeededYou
			}
		}
		for _, hour := range activity.BusyByHour {
			index := slices.IndexFunc(result.BusyByHour, func(item agentinstance.ActivityHour) bool {
				return item.Weekday == hour.Weekday && item.Hour == hour.Hour
			})
			if index < 0 {
				result.BusyByHour = append(result.BusyByHour, hour)
			} else {
				result.BusyByHour[index].Events += hour.Events
			}
		}
	}
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
	c.JSON(http.StatusOK, result)
}
