package core

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) GetAgentInstanceAway(c *gin.Context) {
	if !h.requireOperatorSurface(c, "instance away") {
		return
	}
	instance, ok := h.instanceTarget(c, "reports")
	if !ok {
		return
	}

	response, err := h.AgentInstances.Away(c.Request.Context(), instance.ID)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (api *AgentInstanceAPI) Away(ctx context.Context, id string) (contract.AgentInstanceAwayResponse, error) {
	response := contract.AgentInstanceAwayResponse{Reports: []contract.AgentInstanceReportPayload{}}
	reader, ok := api.Store.(agentinstance.SeenStore)
	if !ok {
		return response, errors.New("instance seen storage unavailable")
	}
	seen, err := reader.LastSeen(ctx, id, agentinstance.LocalOperator)
	if err != nil {
		return response, err
	}
	response.LastSeenAt = seen
	checks, ok := api.Store.(agentinstance.CheckInReader)
	if !ok {
		return response, errors.New("instance check-in storage unavailable")
	}
	quiet, err := checks.CountQuietCheckIns(ctx, id, seen)
	if err != nil {
		return response, err
	}
	response.QuietCheckIns = int(quiet)
	query := agentinstance.ReportQuery{InstanceID: id, Limit: 100}
	if seen != nil {
		query.AfterCreatedAt = *seen
	}
	for {
		reports, err := api.Reports.ListReports(ctx, query)
		if err != nil {
			return response, err
		}
		for _, report := range reports {
			if seen != nil && !report.CreatedAt.After(*seen) {
				continue
			}
			var payload contract.AgentInstanceReportPayload
			if err := json.Unmarshal(report.Payload, &payload); err != nil {
				return response, err
			}
			payload.ID, payload.Kind, payload.State, payload.CreatedAt = report.ID, report.Kind, report.State, report.CreatedAt
			response.Reports = append(response.Reports, payload)
		}
		if len(reports) < query.Limit {
			return response, nil
		}
		last := reports[len(reports)-1]
		query.AfterCreatedAt, query.AfterID = last.CreatedAt, last.ID
	}
}

func (h *BaseHandlers) MarkAgentInstanceSeen(c *gin.Context) {
	if !h.requireOperatorSurface(c, "instance mark seen") {
		return
	}
	instance, ok := h.instanceTarget(c, "reports")
	if !ok {
		return
	}
	reader, ok := h.AgentInstances.Store.(agentinstance.SeenStore)
	if !ok {
		h.respondError(c, 503, errors.New("instance seen storage unavailable"))
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	at, err := reader.MarkSeen(c.Request.Context(), instance.ID, agentinstance.LocalOperator, now)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentInstanceSeenResponse{LastSeenAt: at})
}

func (h *BaseHandlers) ListAgentInstanceCheckIns(c *gin.Context) {
	instance, ok := h.instanceTarget(c, "reports")
	if !ok {
		return
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			h.respondError(c, 422, errors.New("limit must be from 1 to 100"))
			return
		}
		limit = value
	}
	response, err := h.AgentInstances.CheckIns(c.Request.Context(), instance.ID, c.Query("cursor"), limit)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (api *AgentInstanceAPI) CheckIns(ctx context.Context, id, cursor string, limit int) (contract.AgentInstanceCheckInsResponse, error) {
	response := contract.AgentInstanceCheckInsResponse{CheckIns: []contract.AgentInstanceCheckInPayload{}}
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return response, errors.New("limit must be from 1 to 100")
	}
	reader, ok := api.Store.(agentinstance.CheckInReader)
	if !ok {
		return response, errors.New("instance check-in storage unavailable")
	}
	rows, err := reader.ListCheckIns(ctx, id, cursor, limit+1)
	if err != nil {
		return response, err
	}
	if len(rows) > limit {
		rows = rows[:limit]
		response.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		response.CheckIns = append(response.CheckIns, contract.AgentInstanceCheckInPayload{ID: row.ID, Result: row.Result, Reason: row.Reason, CreatedAt: row.CreatedAt})
	}
	return response, nil
}
