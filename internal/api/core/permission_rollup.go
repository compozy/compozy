package core

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type permissionRollupObserver interface {
	QueryPermissionDailyRollups(context.Context, store.PermissionRollupQuery) ([]store.PermissionDailyRollup, error)
	QueryPermissionEntries(context.Context, store.PermissionEntriesQuery) ([]store.PermissionLogEntry, error)
}

func (h *BaseHandlers) PermissionRollup(c *gin.Context) {
	if !h.requireOperatorOrGrantedAgent(c, "permission rollups") {
		return
	}
	observer, query, ok := h.permissionAuditQuery(c)
	if !ok {
		return
	}
	since, now := query.Since, query.Until
	rows, err := observer.QueryPermissionDailyRollups(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	payload := contract.PermissionRollupResponse{
		Since:   since,
		Until:   now,
		Rollups: make([]contract.PermissionDailyRollupPayload, 0, len(rows)),
	}
	for _, row := range rows {
		payload.Rollups = append(
			payload.Rollups,
			contract.PermissionDailyRollupPayload{
				Day:       row.Day,
				AgentName: row.AgentName,
				Decision:  row.Decision,
				Count:     row.Count,
			},
		)
	}
	c.JSON(http.StatusOK, payload)
}

// permissionEntriesLimit bounds the audit list; the rollups stay the exact totals.
const permissionEntriesLimit = 1000

// PermissionEntries lists the newest permission decisions with their initiators.
func (h *BaseHandlers) PermissionEntries(c *gin.Context) {
	if !h.requireOperatorOrGrantedAgent(c, "permission entries") {
		return
	}
	observer, query, ok := h.permissionAuditQuery(c)
	if !ok {
		return
	}
	rows, err := observer.QueryPermissionEntries(
		c.Request.Context(),
		store.PermissionEntriesQuery{PermissionRollupQuery: query, Limit: permissionEntriesLimit},
	)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	payload := contract.PermissionEntriesResponse{Entries: make([]contract.PermissionEntryPayload, 0, len(rows))}
	for _, row := range rows {
		payload.Entries = append(payload.Entries, contract.PermissionEntryPayload{
			ID:            row.ID,
			SessionID:     row.SessionID,
			AgentName:     row.AgentName,
			Action:        row.Action,
			Resource:      row.Resource,
			Decision:      row.Decision,
			InitiatorKind: row.InitiatorKind,
			InitiatorID:   row.InitiatorID,
			Timestamp:     row.Timestamp,
		})
	}
	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) permissionAuditQuery(
	c *gin.Context,
) (permissionRollupObserver, store.PermissionRollupQuery, bool) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return nil, store.PermissionRollupQuery{}, false
	}
	days := 30
	if raw, present := c.GetQuery("days"); present {
		days, err = strconv.Atoi(raw)
		if err != nil || days < 1 || days > 90 {
			h.respondError(c, http.StatusBadRequest, errors.New("days must be between 1 and 90"))
			return nil, store.PermissionRollupQuery{}, false
		}
	}
	unattended, err := parseBoolQuery(c, "unattended")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return nil, store.PermissionRollupQuery{}, false
	}
	observer, ok := h.Observer.(permissionRollupObserver)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: permission rollups unavailable"))
		return nil, store.PermissionRollupQuery{}, false
	}
	now := h.Now().UTC()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	return observer, store.PermissionRollupQuery{
		ReadScope:  scope,
		AgentName:  c.Query("agent"),
		Unattended: unattended,
		Since:      since,
		Until:      now,
	}, true
}
