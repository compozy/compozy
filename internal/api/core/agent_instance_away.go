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
	if _, ok := h.instanceTarget(c, "reports"); !ok {
		return
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			h.respondError(c, 422, errors.New("limit must be from 1 to 100"))
			return
		}
	}
	if c.Query("cursor") != "" {
		h.respondError(c, 422, errors.New("invalid check-in cursor"))
		return
	}
	c.JSON(http.StatusOK, contract.AgentInstanceCheckInsResponse{CheckIns: []contract.AgentInstanceCheckInPayload{}})
}
