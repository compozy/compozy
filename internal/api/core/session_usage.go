package core

import (
	"errors"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

// SessionUsage returns the aggregated token-usage summary for one session.
//
// The summary is sourced from the daemon's authoritative per-session token-stats
// aggregate (populated by the observer as usage-bearing agent events are seen), so
// the web inspector Usage tab renders real data instead of a structurally empty
// metric surface. A session that never reported usage yields an empty summary
// (turn_count 0, absent token/cost fields) rather than a fabricated zero grid.
func (h *BaseHandlers) SessionUsage(c *gin.Context) {
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	if h.Observer == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: observer is required"))
		return
	}
	stats, err := h.Observer.QueryTokenStats(
		c.Request.Context(),
		store.TokenStatsQuery{SessionID: sessionID},
	)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionUsageResponse{Usage: aggregateSessionUsage(stats)})
}

// aggregateSessionUsage folds every per-agent token-stats row for one session into
// a single session-level summary. Sessions normally carry one agent row, but the
// stats table is keyed by (session_id, agent_name), so multiple rows are summed
// defensively. Token/cost fields stay absent unless at least one row reported them.
func aggregateSessionUsage(stats []store.TokenStats) contract.SessionUsagePayload {
	payload := contract.SessionUsagePayload{}
	for i := range stats {
		stat := stats[i]
		payload.InputTokens = addOptionalInt64(payload.InputTokens, stat.InputTokens)
		payload.OutputTokens = addOptionalInt64(payload.OutputTokens, stat.OutputTokens)
		payload.TotalTokens = addOptionalInt64(payload.TotalTokens, stat.TotalTokens)
		payload.TurnCount += stat.TurnCount
	}
	cost := store.AggregateTokenStatsCost(stats)
	payload.TotalCost = cost.TotalCost
	if cost.Currency != nil {
		payload.CostCurrency = *cost.Currency
	}
	payload.CostStatus = contract.CostStatus(cost.Status)
	payload.CostSource = contract.CostSource(cost.Source)
	return payload
}

func addOptionalInt64(acc *int64, delta *int64) *int64 {
	if delta == nil {
		return acc
	}
	if acc == nil {
		total := *delta
		return &total
	}
	total := *acc + *delta
	return &total
}
