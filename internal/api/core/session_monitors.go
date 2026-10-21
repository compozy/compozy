package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/gin-gonic/gin"
)

var errSessionMonitorsUnavailable = errors.New("api: session monitors are unavailable")

// SessionMonitorService reads and cancels the daemon-owned monitors of one session.
type SessionMonitorService interface {
	List(sessionID string) []monitorpkg.Monitor
	Cancel(sessionID, monitorID string) (monitorpkg.Monitor, error)
	ActiveCounts(sessionIDs []string) map[string]int
}

// ListSessionMonitors returns the session's monitors, active first.
func (h *BaseHandlers) ListSessionMonitors(c *gin.Context) {
	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) {
		return
	}
	if h.SessionMonitors == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSessionMonitorsUnavailable)
		return
	}
	c.JSON(http.StatusOK, contract.SessionMonitorsResponse{
		Monitors: contract.SessionMonitorsFromDomain(h.SessionMonitors.List(sessionID)),
	})
}

// CancelSessionMonitor stops one monitor without waking its session.
func (h *BaseHandlers) CancelSessionMonitor(c *gin.Context) {
	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) {
		return
	}
	if h.SessionMonitors == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSessionMonitorsUnavailable)
		return
	}
	canceled, err := h.SessionMonitors.Cancel(sessionID, strings.TrimSpace(c.Param("monitor_id")))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, monitorpkg.ErrNotFound) {
			status = http.StatusNotFound
		}
		h.respondError(c, status, err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionMonitorResponse{Monitor: contract.SessionMonitorFromDomain(canceled)})
}

func (h *BaseHandlers) decorateMonitorSummaries(payloads []contract.SessionPayload) {
	if h.SessionMonitors == nil || len(payloads) == 0 {
		return
	}
	ids := make([]string, 0, len(payloads))
	for i := range payloads {
		ids = append(ids, payloads[i].ID)
	}
	counts := h.SessionMonitors.ActiveCounts(ids)
	for i := range payloads {
		payloads[i].MonitorSummary = contract.SessionMonitorSummaryFromCount(counts[payloads[i].ID])
	}
}
