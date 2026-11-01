package core

import (
	"context"
	"errors"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type permissionRollupObserver interface {
	QueryPermissionDailyRollups(context.Context, store.PermissionRollupQuery) ([]store.PermissionDailyRollup, error)
}

func (h *BaseHandlers) PermissionRollup(c *gin.Context) {
	if !h.requireOperatorOrGrantedAgent(c, "permission rollups") {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	days := 30
	if raw, present := c.GetQuery("days"); present {
		days, err = strconv.Atoi(raw)
		if err != nil || days < 1 || days > 90 {
			h.respondError(c, http.StatusBadRequest, errors.New("days must be between 1 and 90"))
			return
		}
	}
	unattended, err := parseBoolQuery(c, "unattended")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	observer, ok := h.Observer.(permissionRollupObserver)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: permission rollups unavailable"))
		return
	}
	now := h.Now().UTC()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	rows, err := observer.QueryPermissionDailyRollups(c.Request.Context(), store.PermissionRollupQuery{ReadScope: scope, AgentName: c.Query("agent"), Unattended: unattended, Since: since, Until: now})
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	payload := contract.PermissionRollupResponse{Since: since, Until: now, Rollups: make([]contract.PermissionDailyRollupPayload, 0, len(rows))}
	for _, row := range rows {
		payload.Rollups = append(payload.Rollups, contract.PermissionDailyRollupPayload{Day: row.Day, AgentName: row.AgentName, Decision: row.Decision, Count: row.Count})
	}
	c.JSON(http.StatusOK, payload)
}
