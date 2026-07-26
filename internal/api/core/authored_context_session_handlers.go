package core

import (
	"errors"

	"net/http"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/heartbeat"

	"github.com/compozy/agh/internal/soul"

	"github.com/gin-gonic/gin"
)

// GetSessionHealth returns metadata-only session health and wake eligibility.
func (h *BaseHandlers) GetSessionHealth(c *gin.Context) {
	health, ok := h.sessionHealthPayloadForRoute(c)
	if !ok {
		return
	}
	h.respondAuthoredJSON(c, http.StatusOK, contract.SessionHealthResponse{Health: health})
}

// GetSessionStatus returns compact session health with wake state when available.
func (h *BaseHandlers) GetSessionStatus(c *gin.Context) {
	health, ok := h.sessionHealthPayloadForRoute(c)
	if !ok {
		return
	}
	response := contract.SessionStatusResponse{
		SessionID:           health.SessionID,
		WorkspaceID:         health.WorkspaceID,
		AgentName:           health.AgentName,
		State:               health.State,
		Health:              health.Health,
		ActivePrompt:        health.ActivePrompt,
		Attachable:          health.Attachable,
		EligibleForWake:     health.EligibleForWake,
		IneligibilityReason: health.IneligibilityReason,
		UpdatedAt:           health.UpdatedAt,
	}
	if h.HeartbeatStatus != nil {
		status, err := h.availableHeartbeatStatusForHealth(c.Request.Context(), health, false)
		if err != nil {
			h.respondError(c, StatusForHeartbeatError(err), err)
			return
		}
		if status != nil {
			response.WakeState = status.WakeState
		}
	}
	h.respondAuthoredJSON(c, http.StatusOK, response)
}

// InspectSession returns detailed health, wake audit, and policy correlation metadata.
func (h *BaseHandlers) InspectSession(c *gin.Context) {
	includeEvents, err := parseBoolQuery(c, "include_recent_wake_events")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	health, ok := h.sessionHealthPayloadForRoute(c)
	if !ok {
		return
	}
	response := contract.SessionInspectResponse{
		SessionID: health.SessionID,
		Health:    health,
	}
	if h.HeartbeatStatus != nil {
		status, statusErr := h.availableHeartbeatStatusForHealth(c.Request.Context(), health, true)
		if statusErr != nil {
			h.respondError(c, StatusForHeartbeatError(statusErr), statusErr)
			return
		}
		if status != nil {
			response.WakeState = status.WakeState
			response.PolicyDigest = status.Digest
			response.ConfigDigest = status.ConfigDigest
			response.Diagnostics = status.Diagnostics
		}
	}
	if includeEvents {
		target, targetErr := h.resolveAuthoredAgentTarget(c.Request.Context(), health.WorkspaceID, health.AgentName)
		if targetErr != nil {
			h.respondError(c, StatusForHeartbeatError(targetErr), targetErr)
			return
		}
		events, eventsErr := h.heartbeatWakeEvents(c.Request.Context(), target, health.SessionID)
		if eventsErr != nil {
			h.respondError(c, StatusForHeartbeatError(eventsErr), eventsErr)
			return
		}
		response.WakeEvents = events
	}
	h.respondAuthoredJSON(c, http.StatusOK, response)
}

func (h *BaseHandlers) inspectSoulTarget(c *gin.Context, target authoredAgentTarget) {
	if target.packageOwned {
		resolved, err := target.resolvePackageOwnedSoul(c.Request.Context(), nil)
		h.respondSoulPayload(c, target.agentName, &resolved, target.soulConfigProvenance(), err)
		return
	}
	if h.SoulAuthoring == nil {
		h.respondError(c, StatusForSoulError(errSoulAuthoringUnavailable), errSoulAuthoringUnavailable)
		return
	}
	result, err := h.SoulAuthoring.Validate(c.Request.Context(), soul.ValidateRequest{
		Target: target.soulAuthoringTarget(),
	})
	h.respondSoulPayload(c, target.agentName, &result.Soul, target.soulConfigProvenance(), err)
}

func (h *BaseHandlers) validateSoulTarget(c *gin.Context, target authoredAgentTarget, body string) {
	if target.packageOwned {
		var bodyPtr *string
		if strings.TrimSpace(body) != "" {
			bodyPtr = &body
		}
		resolved, err := target.resolvePackageOwnedSoul(c.Request.Context(), bodyPtr)
		h.respondSoulPayload(c, target.agentName, &resolved, target.soulConfigProvenance(), err)
		return
	}
	if h.SoulAuthoring == nil {
		h.respondError(c, StatusForSoulError(errSoulAuthoringUnavailable), errSoulAuthoringUnavailable)
		return
	}
	var bodyPtr *string
	if strings.TrimSpace(body) != "" {
		bodyPtr = &body
	}
	result, err := h.SoulAuthoring.Validate(c.Request.Context(), soul.ValidateRequest{
		Target: target.soulAuthoringTarget(),
		Body:   bodyPtr,
	})
	h.respondSoulPayload(c, target.agentName, &result.Soul, target.soulConfigProvenance(), err)
}

func (h *BaseHandlers) inspectHeartbeatTarget(c *gin.Context, target authoredAgentTarget) {
	if target.packageOwned && h.HeartbeatStatus == nil {
		policy, err := target.resolvePackageOwnedHeartbeat(c.Request.Context(), nil)
		h.respondHeartbeatPolicy(c, target.agentName, &policy, "", err)
		return
	}
	if h.HeartbeatStatus == nil {
		h.respondError(c, StatusForHeartbeatError(errHeartbeatStatusMissing), errHeartbeatStatusMissing)
		return
	}
	result, err := h.HeartbeatStatus.Inspect(c.Request.Context(), heartbeat.InspectRequest{
		Target: target.heartbeatAuthoringTarget(),
	})
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	snapshotID := ""
	if result.Snapshot != nil {
		snapshotID = result.Snapshot.ID
	}
	h.respondHeartbeatPolicy(c, result.AgentName, &result.Policy, snapshotID, nil)
}

func (h *BaseHandlers) respondSoulPayload(
	c *gin.Context,
	agentName string,
	resolved *soul.ResolvedSoul,
	provenance soul.ConfigProvenance,
	err error,
) {
	if err != nil && !errors.Is(err, soul.ErrInvalid) {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	payload := contract.AgentSoulPayloadFromResolved(agentName, resolved, "", provenance)
	status := http.StatusOK
	if err != nil || resolved == nil || !resolved.Valid {
		status = http.StatusUnprocessableEntity
	}
	h.respondAuthoredJSON(c, status, payload)
}

func (h *BaseHandlers) respondSoulMutation(
	c *gin.Context,
	target authoredAgentTarget,
	result *soul.MutationResult,
	err error,
) {
	if err != nil {
		if errors.Is(err, soul.ErrInvalid) {
			if result != nil {
				h.respondSoulPayload(c, target.agentName, &result.Soul, target.soulConfigProvenance(), err)
				return
			}
		}
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	response, err := soulMutationResponse(target, result)
	if err != nil {
		h.respondError(c, StatusForSoulError(err), err)
		return
	}
	h.respondAuthoredJSON(c, http.StatusOK, response)
}

func (h *BaseHandlers) respondHeartbeatPolicy(
	c *gin.Context,
	agentName string,
	policy *heartbeat.ResolvedPolicy,
	snapshotID string,
	err error,
) {
	if err != nil && !errors.Is(err, heartbeat.ErrInvalid) {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	payload, convertErr := contract.HeartbeatPolicyPayloadFromResolved(agentName, policy, snapshotID)
	if convertErr != nil {
		h.respondError(c, StatusForHeartbeatError(convertErr), convertErr)
		return
	}
	status := http.StatusOK
	if err != nil || policy == nil || !policy.Valid {
		status = http.StatusUnprocessableEntity
	}
	h.respondAuthoredJSON(c, status, payload)
}

func (h *BaseHandlers) respondHeartbeatMutation(
	c *gin.Context,
	target authoredAgentTarget,
	result *heartbeat.MutationResult,
	err error,
) {
	if err != nil {
		if errors.Is(err, heartbeat.ErrInvalid) {
			if result != nil {
				h.respondHeartbeatPolicy(c, target.agentName, &result.Policy, "", err)
				return
			}
		}
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	response, err := heartbeatMutationResponse(result)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}
	h.respondAuthoredJSON(c, http.StatusOK, response)
}

func (h *BaseHandlers) respondAuthoredJSON(c *gin.Context, status int, payload any) {
	if err := contract.ValidateAuthoredContextRedacted(payload); err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(status, payload)
}
