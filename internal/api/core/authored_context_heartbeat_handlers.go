package core

import (
	"net/http"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/gin-gonic/gin"
)

// GetAgentHeartbeat returns the resolved Heartbeat policy for one workspace-visible agent.
func (h *BaseHandlers) GetAgentHeartbeat(c *gin.Context) {
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		authoredWorkspaceRefFromQuery(c),
		pathAgentName(c),
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	h.inspectHeartbeatTarget(c, target)
}

// ValidateAgentHeartbeat validates a proposed HEARTBEAT.md body.
func (h *BaseHandlers) ValidateAgentHeartbeat(c *gin.Context) {
	if h.HeartbeatAuthoring == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatAuthoringMissing), errHeartbeatAuthoringMissing)
		return
	}
	var req contract.HeartbeatValidateByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	if target.packageOwned {
		body := req.Body
		policy, policyErr := target.resolvePackageOwnedHeartbeat(c.Request.Context(), &body)
		h.respondHeartbeatPolicy(c, target.agentName, &policy, "", policyErr)
		return
	}
	body := req.Body
	result, err := h.HeartbeatAuthoring.Validate(c.Request.Context(), heartbeat.ValidateRequest{
		Target: target.heartbeatAuthoringTarget(),
		Body:   &body,
	})
	h.respondHeartbeatPolicy(c, target.agentName, &result.Policy, "", err)
}

// PutAgentHeartbeat creates or replaces HEARTBEAT.md through the managed authoring service.
func (h *BaseHandlers) PutAgentHeartbeat(c *gin.Context) {
	if h.rejectHeartbeatIfMatch(c) {
		return
	}
	if h.HeartbeatAuthoring == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatAuthoringMissing), errHeartbeatAuthoringMissing)
		return
	}
	var req contract.HeartbeatPutByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	if err := target.rejectPackageOwnedHeartbeatMutation(); err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	result, err := h.HeartbeatAuthoring.Put(c.Request.Context(), heartbeat.PutRequest{
		Target:         target.heartbeatAuthoringTarget(),
		Body:           req.Body,
		ExpectedDigest: strings.TrimSpace(req.ExpectedDigest),
		Actor:          h.heartbeatActorForRequest(),
	})
	h.respondHeartbeatMutation(c, target, &result, err)
}

// DeleteAgentHeartbeat removes HEARTBEAT.md through the managed authoring service.
func (h *BaseHandlers) DeleteAgentHeartbeat(c *gin.Context) {
	if h.rejectHeartbeatIfMatch(c) {
		return
	}
	if h.HeartbeatAuthoring == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatAuthoringMissing), errHeartbeatAuthoringMissing)
		return
	}
	var req contract.HeartbeatDeleteByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	if err := target.rejectPackageOwnedHeartbeatMutation(); err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	result, err := h.HeartbeatAuthoring.Delete(c.Request.Context(), heartbeat.DeleteRequest{
		Target:         target.heartbeatAuthoringTarget(),
		ExpectedDigest: strings.TrimSpace(req.ExpectedDigest),
		Actor:          h.heartbeatActorForRequest(),
	})
	h.respondHeartbeatMutation(c, target, &result, err)
}

// ListAgentHeartbeatHistory lists HEARTBEAT.md managed authoring revisions.
func (h *BaseHandlers) ListAgentHeartbeatHistory(c *gin.Context) {
	if h.HeartbeatAuthoring == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatAuthoringMissing), errHeartbeatAuthoringMissing)
		return
	}
	limit, err := parsePositiveIntQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		authoredWorkspaceRefFromQuery(c),
		pathAgentName(c),
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	if target.packageOwned {
		response := contract.HeartbeatHistoryResponse{
			Revisions: []contract.HeartbeatRevisionPayload{},
		}
		h.respondAuthoredJSON(
			c,
			http.StatusOK,
			response,
		)
		return
	}
	result, err := h.HeartbeatAuthoring.History(c.Request.Context(), heartbeat.HistoryRequest{
		Target: target.heartbeatAuthoringTarget(),
		Limit:  limit,
	})
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	response, err := heartbeatHistoryResponse(result)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	h.respondAuthoredJSON(c, http.StatusOK, response)
}

// RollbackAgentHeartbeat rolls HEARTBEAT.md back to a prior revision or snapshot digest.
func (h *BaseHandlers) RollbackAgentHeartbeat(c *gin.Context) {
	if h.rejectHeartbeatIfMatch(c) {
		return
	}
	if h.HeartbeatAuthoring == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatAuthoringMissing), errHeartbeatAuthoringMissing)
		return
	}
	var req contract.HeartbeatRollbackByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	if err := target.rejectPackageOwnedHeartbeatMutation(); err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	result, err := h.HeartbeatAuthoring.Rollback(c.Request.Context(), heartbeat.RollbackRequest{
		Target:         target.heartbeatAuthoringTarget(),
		RevisionID:     strings.TrimSpace(req.RevisionID),
		TargetDigest:   strings.TrimSpace(req.TargetDigest),
		ExpectedDigest: strings.TrimSpace(req.ExpectedDigest),
		Actor:          h.heartbeatActorForRequest(),
	})
	h.respondHeartbeatMutation(c, target, &result, err)
}

// GetAgentHeartbeatStatus returns policy, wake state, and optional session health.
func (h *BaseHandlers) GetAgentHeartbeatStatus(c *gin.Context) {
	if h.HeartbeatStatus == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatStatusMissing), errHeartbeatStatusMissing)
		return
	}
	includeHealth, err := parseBoolQuery(c, "include_session_health")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	includeEvents, err := parseBoolQuery(c, "include_recent_wake_events")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		authoredWorkspaceRefFromQuery(c),
		pathAgentName(c),
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID != "" {
		if _, err := h.requireSessionInWorkspace(
			c.Request.Context(),
			target.storageWorkspaceID(),
			sessionID,
		); err != nil {
			h.respondError(c, statusForWorkspaceScopedResourceError(err), err)
			return
		}
	}
	result, err := h.HeartbeatStatus.Status(c.Request.Context(), heartbeat.StatusRequest{
		Target:               target.heartbeatAuthoringTarget(),
		SessionID:            sessionID,
		IncludeSessionHealth: includeHealth,
	})
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	response, err := contract.HeartbeatStatusResponseFromResult(&result)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	if includeEvents {
		events, eventsErr := h.heartbeatWakeEvents(c.Request.Context(), target, sessionID)
		if eventsErr != nil {
			h.respondError(c, StatusForHeartbeatError(eventsErr), eventsErr)
			return
		}
		response.WakeEvents = events
	}
	h.respondAuthoredJSON(c, http.StatusOK, response)
}

// WakeAgentHeartbeat requests one manual advisory Heartbeat wake decision.
func (h *BaseHandlers) WakeAgentHeartbeat(c *gin.Context) {
	if h.HeartbeatWake == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatWakeMissing), errHeartbeatWakeMissing)
		return
	}
	var req contract.HeartbeatWakeByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	source := heartbeat.WakeSource(req.Source)
	if source == "" {
		source = heartbeat.WakeSourceManual
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID != "" {
		if _, err := h.requireSessionInWorkspace(
			c.Request.Context(),
			target.storageWorkspaceID(),
			sessionID,
		); err != nil {
			h.respondError(c, statusForWorkspaceScopedResourceError(err), err)
			return
		}
	}
	decision, err := h.HeartbeatWake.Wake(c.Request.Context(), heartbeat.WakeRequest{
		WorkspaceID: target.storageWorkspaceID(),
		AgentName:   target.agentName,
		SessionID:   sessionID,
		Source:      source,
		DryRun:      req.DryRun,
	})
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	payload, err := contract.HeartbeatWakeDecisionPayloadFromDomain(decision)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	status := statusForHeartbeatWakeDecision(decision)
	h.respondAuthoredJSON(c, status, contract.HeartbeatWakeResponse{Decision: payload})
}
