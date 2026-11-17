package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/workspaceaccess"
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
	_, sessionID, _, ok := h.routeScopedSessionInWorkspace(c)
	if !ok {
		return
	}
	if h.SessionMonitors == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSessionMonitorsUnavailable)
		return
	}
	viewer, ok := h.monitorViewer(c)
	if !ok {
		return
	}
	monitors := h.SessionMonitors.List(sessionID)
	for i := range monitors {
		monitors[i] = h.visibleSessionMonitor(c, viewer, monitors[i])
	}
	c.JSON(http.StatusOK, contract.SessionMonitorsResponse{
		Monitors: contract.SessionMonitorsFromDomain(monitors),
	})
}

// CancelSessionMonitor stops one monitor without waking its session.
func (h *BaseHandlers) CancelSessionMonitor(c *gin.Context) {
	_, sessionID, _, ok := h.routeScopedSessionInWorkspace(c)
	if !ok {
		return
	}
	if h.SessionMonitors == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSessionMonitorsUnavailable)
		return
	}
	viewer, ok := h.monitorViewer(c)
	if !ok {
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
	canceled = h.visibleSessionMonitor(c, viewer, canceled)
	c.JSON(http.StatusOK, contract.SessionMonitorResponse{Monitor: contract.SessionMonitorFromDomain(canceled)})
}

func (h *BaseHandlers) monitorViewer(c *gin.Context) (*session.Info, bool) {
	if !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) {
		return nil, true
	}
	caller, ok := h.requireAgentCaller(c, "session monitors")
	if !ok {
		return nil, false
	}
	return &session.Info{ID: caller.Session.ID, AgentName: caller.Session.AgentName,
		WorkspaceID: caller.Session.WorkspaceID, ProfileID: caller.Session.ProfileID}, true
}

func (h *BaseHandlers) visibleSessionMonitor(
	c *gin.Context, viewer *session.Info, mon monitorpkg.Monitor,
) monitorpkg.Monitor {
	if viewer == nil || mon.Kind != monitorpkg.KindSession || len(mon.Targets) == 0 {
		return mon
	}
	var denied bool
	mon.Targets, denied = VisibleMonitorTargets(
		c.Request.Context(), h.Sessions, h.WorkspaceAccess, viewer, mon.Targets, workspaceaccess.SeamIdentity,
	)
	if denied {
		mon.Reason = "access_denied"
	}
	return mon
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
