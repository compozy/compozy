package core

import (
	"errors"

	"net/http"
	"os"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

const defaultWakeEventInspectLimit = 10

var (
	errAuthoredContextValidation = errors.New("authored context validation error")
	errSoulAuthoringUnavailable  = errors.New("api: soul authoring service is not configured")
	errSoulRefreshUnavailable    = errors.New("api: soul refresh service is not configured")
	errHeartbeatAuthoringMissing = errors.New("api: heartbeat authoring service is not configured")
	errHeartbeatStatusMissing    = errors.New("api: heartbeat status service is not configured")
	errHeartbeatWakeMissing      = errors.New("api: heartbeat wake service is not configured")
	errSessionHealthMissing      = errors.New("api: session health service is not configured")
)

type authoredAgentTarget struct {
	workspaceID         string
	sessionWorkspaceID  string
	workspaceRoot       string
	agentName           string
	agentPath           string
	soulConfig          aghconfig.SoulConfig
	heartbeatConfig     aghconfig.HeartbeatConfig
	packageOwned        bool
	soulSourcePath      string
	soulBody            string
	heartbeatSourcePath string
	heartbeatBody       string
}

// StatusForSoulError maps Soul/session authoring failures to deterministic transport statuses.
func StatusForSoulError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, errSoulAuthoringUnavailable),
		errors.Is(err, errSoulRefreshUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, errAuthoredContextValidation):
		return http.StatusBadRequest
	case errors.Is(err, soul.ErrAuthoringConflict),
		errors.Is(err, session.ErrSoulRefreshConflict),
		errors.Is(err, session.ErrSoulRefreshDigestConflict),
		errors.Is(err, session.ErrSessionNotActive):
		return http.StatusConflict
	case errors.Is(err, soul.ErrAuthoringAgentNotFound),
		errors.Is(err, soul.ErrAuthoringMissing),
		errors.Is(err, soul.ErrRevisionNotFound),
		errors.Is(err, soul.ErrSnapshotNotFound),
		errors.Is(err, session.ErrSessionNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return http.StatusGone
	case errors.Is(err, soul.ErrInvalid),
		errors.Is(err, soul.ErrInvalidSnapshot),
		errors.Is(err, soul.ErrInvalidRevision),
		errors.Is(err, soul.ErrAuthoringPathRejected),
		errors.Is(err, contract.ErrInvalidAuthoredContextEnum):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// StatusForHeartbeatError maps Heartbeat authoring/status failures to deterministic transport statuses.
func StatusForHeartbeatError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, errHeartbeatAuthoringMissing),
		errors.Is(err, errHeartbeatStatusMissing),
		errors.Is(err, errHeartbeatWakeMissing),
		errors.Is(err, errSessionHealthMissing):
		return http.StatusServiceUnavailable
	case errors.Is(err, errAuthoredContextValidation):
		return http.StatusBadRequest
	case errors.Is(err, heartbeat.ErrAuthoringConflict):
		return http.StatusConflict
	case errors.Is(err, heartbeat.ErrAuthoringAgentNotFound),
		errors.Is(err, heartbeat.ErrAuthoringNoPolicy),
		errors.Is(err, heartbeat.ErrRevisionNotFound),
		errors.Is(err, heartbeat.ErrSnapshotNotFound),
		errors.Is(err, heartbeat.ErrSessionHealthNotFound),
		errors.Is(err, session.ErrSessionNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return http.StatusGone
	case errors.Is(err, heartbeat.ErrInvalid),
		errors.Is(err, heartbeat.ErrInvalidSnapshot),
		errors.Is(err, heartbeat.ErrInvalidRevision),
		errors.Is(err, heartbeat.ErrInvalidSessionHealth),
		errors.Is(err, heartbeat.ErrInvalidWakeEvent),
		errors.Is(err, heartbeat.ErrInvalidWakeState),
		errors.Is(err, heartbeat.ErrAuthoringPathRejected),
		errors.Is(err, contract.ErrInvalidAuthoredContextEnum):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// AgentSoul returns the caller's resolved full Soul read model.
func (h *BaseHandlers) AgentSoul(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, "agent.soul.inspect")
	if !ok {
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		caller.Session.WorkspaceID,
		caller.Session.AgentName,
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	h.inspectSoulTarget(c, target)
}

// GetAgentSoul returns the resolved full Soul read model for one workspace-visible agent.
func (h *BaseHandlers) GetAgentSoul(c *gin.Context) {
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		authoredWorkspaceRefFromQuery(c),
		pathAgentName(c),
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	h.inspectSoulTarget(c, target)
}

// ValidateAgentSoulDefinition validates a proposed SOUL.md body for one workspace-visible agent.
func (h *BaseHandlers) ValidateAgentSoulDefinition(c *gin.Context) {
	var req contract.AgentSoulValidateByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	h.validateSoulTarget(c, target, req.Body)
}

// ValidateAgentSoul validates a proposed SOUL.md body for the caller's agent.
func (h *BaseHandlers) ValidateAgentSoul(c *gin.Context) {
	caller, ok := h.requireAgentCaller(c, "agent.soul.validate")
	if !ok {
		return
	}
	var req contract.AgentSoulValidateRequest
	if err := decodeAuthoredJSONBody(c, &req, true); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, caller.Session.WorkspaceID),
		firstNonEmpty(req.AgentName, caller.Session.AgentName),
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	h.validateSoulTarget(c, target, req.Body)
}

// PutAgentSoul creates or replaces SOUL.md through the managed authoring service.
func (h *BaseHandlers) PutAgentSoul(c *gin.Context) {
	if h.SoulAuthoring == nil {
		h.respondError(c, StatusForSoulError(errSoulAuthoringUnavailable), errSoulAuthoringUnavailable)
		return
	}
	if h.rejectSoulIfMatch(c) {
		return
	}
	var req contract.AgentSoulPutByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	if err := target.rejectPackageOwnedSoulMutation(); err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	result, err := h.SoulAuthoring.Put(c.Request.Context(), soul.PutRequest{
		Target:         target.soulAuthoringTarget(),
		Body:           req.Body,
		ExpectedDigest: strings.TrimSpace(req.ExpectedDigest),
		Actor:          h.soulActorForRequest(),
		Origin:         h.soulOriginForRequest(c),
	})
	h.respondSoulMutation(c, target, &result, err)
}

// DeleteAgentSoul removes SOUL.md through the managed authoring service.
func (h *BaseHandlers) DeleteAgentSoul(c *gin.Context) {
	if h.SoulAuthoring == nil {
		h.respondError(c, StatusForSoulError(errSoulAuthoringUnavailable), errSoulAuthoringUnavailable)
		return
	}
	if h.rejectSoulIfMatch(c) {
		return
	}
	var req contract.AgentSoulDeleteByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	if err := target.rejectPackageOwnedSoulMutation(); err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	result, err := h.SoulAuthoring.Delete(c.Request.Context(), soul.DeleteRequest{
		Target:         target.soulAuthoringTarget(),
		ExpectedDigest: strings.TrimSpace(req.ExpectedDigest),
		Actor:          h.soulActorForRequest(),
		Origin:         h.soulOriginForRequest(c),
	})
	h.respondSoulMutation(c, target, &result, err)
}

// ListAgentSoulHistory lists SOUL.md managed authoring revisions.
func (h *BaseHandlers) ListAgentSoulHistory(c *gin.Context) {
	if h.SoulAuthoring == nil {
		h.respondError(c, StatusForSoulError(errSoulAuthoringUnavailable), errSoulAuthoringUnavailable)
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
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	if target.packageOwned {
		response := contract.AgentSoulHistoryResponse{
			Revisions: []contract.AgentSoulRevisionPayload{},
		}
		h.respondAuthoredJSON(
			c,
			http.StatusOK,
			response,
		)
		return
	}
	result, err := h.SoulAuthoring.History(c.Request.Context(), soul.HistoryRequest{
		Target: target.soulAuthoringTarget(),
		Limit:  limit,
	})
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	response, err := soulHistoryResponse(result)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	h.respondAuthoredJSON(c, http.StatusOK, response)
}

// RollbackAgentSoul rolls SOUL.md back to a prior managed revision.
func (h *BaseHandlers) RollbackAgentSoul(c *gin.Context) {
	if h.SoulAuthoring == nil {
		h.respondError(c, StatusForSoulError(errSoulAuthoringUnavailable), errSoulAuthoringUnavailable)
		return
	}
	if h.rejectSoulIfMatch(c) {
		return
	}
	var req contract.AgentSoulRollbackByPathRequest
	if err := decodeAuthoredJSONBody(c, &req, false); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	agentName, err := authoredRouteAgentName(pathAgentName(c))
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	target, err := h.resolveAuthoredAgentTarget(
		c.Request.Context(),
		firstNonEmpty(req.WorkspaceID, authoredWorkspaceRefFromQuery(c)),
		agentName,
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	if err := target.rejectPackageOwnedSoulMutation(); err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	result, err := h.SoulAuthoring.Rollback(c.Request.Context(), soul.RollbackRequest{
		Target:         target.soulAuthoringTarget(),
		RevisionID:     strings.TrimSpace(req.RevisionID),
		ExpectedDigest: strings.TrimSpace(req.ExpectedDigest),
		Actor:          h.soulActorForRequest(),
		Origin:         h.soulOriginForRequest(c),
	})
	h.respondSoulMutation(c, target, &result, err)
}

// RefreshSessionSoul refreshes an idle session's Soul snapshot through body-level CAS.
func (h *BaseHandlers) RefreshSessionSoul(c *gin.Context) {
	if h.SoulRefresher == nil {
		h.respondError(c, StatusForSoulError(errSoulRefreshUnavailable), errSoulRefreshUnavailable)
		return
	}
	if h.rejectSoulIfMatch(c) {
		return
	}
	var req contract.SessionSoulRefreshRequest
	if err := decodeAuthoredJSONBody(c, &req, true); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	result, err := h.SoulRefresher.RefreshSoulWithExpectedDigest(
		c.Request.Context(),
		sessionID,
		req.ExpectedDigest,
	)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	payload, err := agentSoulPayloadFromRefreshResult(result)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	h.respondAuthoredJSON(c, http.StatusOK, payload)
}
