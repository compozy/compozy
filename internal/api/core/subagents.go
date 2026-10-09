package core

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) ListSessionSubagents(c *gin.Context) {
	if !h.requireSubagents(c) {
		return
	}
	id := c.Param("session_id")
	workspaceID, ok := h.subagentWorkspace(c, "session_not_found", "Session "+id+" not found.")
	if !ok || !h.authorizeSubagentParent(c, workspaceID, id, false, "session_not_found", "Session "+id+" not found.") {
		return
	}
	query, err := parseSubagentListQuery(c)
	if err != nil {
		subagentError(c, 400, "invalid_request", err.Error())
		return
	}
	query.WorkspaceID, query.ParentSessionID = workspaceID, id
	page, err := h.Subagents.List(c.Request.Context(), query)
	if err != nil {
		h.respondSubagentError(c, err)
		return
	}
	payload := contract.SubagentListPayload{Subagents: subagentPagePayloads(page)}
	if page.NextCursor != "" {
		payload.NextCursor = new(page.NextCursor)
	}
	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) GetSubagent(c *gin.Context) {
	row, ok := h.authorizedSubagent(c, false)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, contract.SubagentFromDomain(row))
}

func (h *BaseHandlers) CancelSubagent(c *gin.Context) {
	row, ok := h.authorizedSubagent(c, true)
	if !ok {
		return
	}
	var request contract.SubagentCancelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		subagentError(c, 400, "invalid_request", "Invalid cancellation request.")
		return
	}
	if row.Origin == store.SubagentOriginProviderNative {
		h.respondSubagentError(c, session.ErrSubagentNotCancelable)
		return
	}
	actor := session.SubagentActor{Kind: "operator"}
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, err := h.resolveAgentCallerForWorkspace(c.Request.Context(), credentials, "subagent.cancel", row.WorkspaceID)
		if err != nil {
			h.respondError(c, StatusForAgentIdentityError(err), err)
			return
		}
		actor = session.SubagentActor{Kind: "agent", ID: caller.Session.ID, Caller: &session.SubagentCaller{
			WorkspaceID: caller.Session.WorkspaceID, SessionID: caller.Session.ID, AgentName: caller.Session.AgentName,
		}}
	}
	outcome, err := h.Subagents.Cancel(c.Request.Context(), actor, row.ID, request.Reason)
	if err != nil {
		h.respondSubagentError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, contract.SubagentCancelPayload{SubagentID: outcome.ID, Status: outcome.Status})
}

func (h *BaseHandlers) requireSubagents(c *gin.Context) bool {
	if h.Subagents == nil {
		subagentError(c, http.StatusServiceUnavailable, "feature_unavailable", "Subagents are unavailable.")
		return false
	}
	return true
}

func (h *BaseHandlers) authorizedSubagent(c *gin.Context, write bool) (session.Subagent, bool) {
	if !h.requireSubagents(c) {
		return session.Subagent{}, false
	}
	id := c.Param("subagent_id")
	message := "Subagent " + id + " not found."
	workspaceID, ok := h.subagentWorkspace(c, "subagent_not_found", message)
	if !ok {
		return session.Subagent{}, false
	}
	row, err := h.Subagents.Get(c.Request.Context(), workspaceID, id)
	if err != nil {
		if errors.Is(err, session.ErrSubagentNotFound) || errors.Is(err, store.ErrSubagentNotFound) {
			subagentError(c, 404, "subagent_not_found", message)
		} else {
			h.respondSubagentError(c, err)
		}
		return session.Subagent{}, false
	}
	if row.WorkspaceID != workspaceID || !h.authorizeSubagentParent(c, workspaceID, row.ParentSessionID, write, "subagent_not_found", message) {
		if row.WorkspaceID != workspaceID {
			subagentError(c, 404, "subagent_not_found", message)
		}
		return session.Subagent{}, false
	}
	return row, true
}

func (h *BaseHandlers) subagentWorkspace(c *gin.Context, code, message string) (string, bool) {
	if h.Workspaces == nil {
		subagentError(c, 503, "feature_unavailable", "Workspace resolver is unavailable.")
		return "", false
	}
	resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspaceRefFromRoute(c))
	if err != nil {
		subagentError(c, 404, code, message)
		return "", false
	}
	return resolved.ID, true
}

func (h *BaseHandlers) authorizeSubagentParent(c *gin.Context, workspaceID, parentID string, write bool, code, message string) bool {
	info, err := h.requireSessionInWorkspace(c.Request.Context(), workspaceID, parentID)
	if err != nil {
		subagentError(c, 404, code, message)
		return false
	}
	var selection resolvedProfileReadScope
	if write {
		selection, err = h.resolveProfileMutationSelectionForWorkspace(c, workspaceID)
	} else {
		selection, err = h.resolveProfileReadSelectionForWorkspace(c, workspaceID)
	}
	if err != nil || !selection.Scope.Matches(info.ProfileID) {
		subagentError(c, 404, code, message)
		return false
	}
	return true
}

func parseSubagentListQuery(c *gin.Context) (store.SubagentListQuery, error) {
	query := store.SubagentListQuery{Limit: 50, Cursor: c.Query("cursor")}
	if raw := c.Query("limit"); c.Request.URL.Query().Has("limit") {
		limit, err := ParseOptionalInt(raw)
		if err != nil || limit < 1 || limit > 200 {
			return query, errors.New("limit must be between 1 and 200.")
		}
		query.Limit = limit
	}
	if origin := c.Query("origin"); origin != "" {
		if origin != store.SubagentOriginDelegated && origin != store.SubagentOriginProviderNative {
			return query, errors.New("origin must be delegated or provider_native.")
		}
		query.Origins = []string{origin}
	}
	if status := c.Query("status"); status != "" {
		for value := range strings.SplitSeq(status, ",") {
			value = strings.TrimSpace(value)
			switch value {
			case store.SubagentStatusQueued, store.SubagentStatusRunning, store.SubagentStatusWaiting,
				store.SubagentStatusCompleted, store.SubagentStatusFailed, store.SubagentStatusCanceled, store.SubagentStatusInterrupted:
				query.Statuses = append(query.Statuses, value)
			default:
				return query, fmt.Errorf("invalid subagent status %q.", value)
			}
		}
		slices.Sort(query.Statuses)
		query.Statuses = slices.Compact(query.Statuses)
	}
	return query, nil
}

func subagentPagePayloads(page store.SubagentPage) []contract.SubagentPayload {
	payloads := make([]contract.SubagentPayload, 0, len(page.Items))
	for _, row := range page.Items {
		payloads = append(payloads, contract.SubagentFromDomain(session.Subagent{SessionSubagent: row}))
	}
	return payloads
}

func subagentError(c *gin.Context, status int, code, message string) {
	c.JSON(status, contract.ErrorPayload{Code: code, Error: message})
}

func (h *BaseHandlers) respondSubagentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, session.ErrSubagentNotFound), errors.Is(err, store.ErrSubagentNotFound):
		subagentError(c, 404, "subagent_not_found", "Subagent "+c.Param("subagent_id")+" not found.")
	case errors.Is(err, session.ErrSubagentNotCancelable):
		subagentError(c, 409, "subagent_not_cancelable", "Provider-native subagents cannot be canceled; stop the parent turn instead.")
	case errors.Is(err, session.ErrSubagentCapabilityDenied):
		subagentError(c, http.StatusForbidden, "capability_denied", err.Error())
	case errors.Is(err, session.ErrSubagentInvalidRequest), errors.Is(err, listcursor.ErrInvalid):
		subagentError(c, 400, "invalid_request", err.Error())
	default:
		h.respondError(c, 500, err)
	}
}
