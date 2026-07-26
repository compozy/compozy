package core

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/compozy/agh/internal/api/contract"
	networkusage "github.com/compozy/agh/internal/network/usage"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

// GetNetworkUsage returns workspace-scoped wake usage from the ledger.
func (h *BaseHandlers) GetNetworkUsage(c *gin.Context) {
	workspaceID, ok := h.requireRouteWorkspaceID(c)
	if !ok {
		return
	}
	if h.NetworkUsage == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: network usage store is unavailable"))
		return
	}
	query, err := ParseNetworkUsageQuery(workspaceID, c.Request.URL.Query())
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	report, err := h.NetworkUsage.GetNetworkUsage(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForNetworkError(err), err)
		return
	}
	response, err := NetworkUsageResponseFromReport(workspaceID, report)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// ParseNetworkUsageQuery delegates every public adapter to one strict query contract.
func ParseNetworkUsageQuery(workspaceID string, values url.Values) (store.NetworkUsageQuery, error) {
	return networkusage.ParseQuery(workspaceID, values)
}

// NetworkUsageResponseFromReport delegates every public adapter to one response projection.
func NetworkUsageResponseFromReport(
	workspaceID string,
	report store.NetworkUsageReport,
) (contract.NetworkUsageResponse, error) {
	return networkusage.ResponseFromReport(workspaceID, report)
}
