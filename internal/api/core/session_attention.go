package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// SessionPresence acquires, renews, or releases one operator presence lease.
func (h *BaseHandlers) SessionPresence(c *gin.Context) {
	if !h.requireOperatorSurface(c, "presence") {
		return
	}
	manager, ok := h.Sessions.(SessionAttentionManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session attention is required"))
		return
	}
	var request contract.SessionPresenceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	request.LeaseID = strings.TrimSpace(request.LeaseID)
	if !request.Visible && request.LeaseID == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("api: presence lease_id is required for release"))
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	leaseID, err := manager.SessionPresence(c.Request.Context(), sessionID, request.LeaseID, request.Visible)
	if err != nil {
		h.respondError(c, statusForPresenceError(err), err)
		return
	}
	if request.Visible && request.LeaseID == "" {
		c.JSON(http.StatusOK, contract.SessionPresenceResponse{LeaseID: leaseID})
		return
	}
	c.Status(http.StatusNoContent)
}

// ListSessionInteractions lists sanitized actionable interactions for one session.
func (h *BaseHandlers) ListSessionInteractions(c *gin.Context) {
	if !h.authorizeSessionInteractionDiscovery(c) {
		return
	}
	manager, ok := h.Sessions.(SessionAttentionManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session attention is required"))
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	statuses := c.QueryArray("status")
	interactions, err := manager.PendingInteractions(c.Request.Context(), sessionID, statuses)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrSessionNotFound) || errors.Is(err, session.ErrSessionNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, store.ErrPendingInteractionStatusInvalid) {
			status = http.StatusBadRequest
		}
		h.respondError(c, status, err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionInteractionsResponse{
		Interactions: PendingInteractionPayloadsFromStore(interactions),
	})
}

func (h *BaseHandlers) authorizeSessionInteractionDiscovery(c *gin.Context) bool {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return true
	}
	_, err := h.resolveAgentCallerForWorkspace(
		c.Request.Context(),
		credentials,
		"session.interactions",
		workspaceRefFromRoute(c),
	)
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return false
	}
	return true
}

// SessionAttentionSummary returns exact operator-wide attention counts.
func (h *BaseHandlers) SessionAttentionSummary(c *gin.Context) {
	if !h.requireOperatorSurface(c, "attention summary") {
		return
	}
	manager, ok := h.Sessions.(SessionAttentionManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session attention is required"))
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	summary, err := manager.AttentionSummary(c.Request.Context(), readScope)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	workspaces := make([]contract.WorkspaceAttentionSummaryPayload, 0, len(summary.ByWorkspace))
	for _, workspace := range summary.ByWorkspace {
		workspaces = append(workspaces, contract.WorkspaceAttentionSummaryPayload{
			WorkspaceID: workspace.WorkspaceID,
			NeedsYou:    workspace.NeedsYou,
			Finished:    workspace.Finished,
		})
	}
	c.JSON(http.StatusOK, contract.SessionAttentionSummaryPayload{
		NeedsYou: summary.NeedsYou, Finished: summary.Finished, ByWorkspace: workspaces,
	})
}

func statusForPresenceError(err error) int {
	switch {
	case errors.Is(err, session.ErrSessionPresenceLeaseNotFound), errors.Is(err, store.ErrSessionNotFound):
		return http.StatusNotFound
	case errors.Is(err, session.ErrSessionPresenceLeaseMismatch):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
