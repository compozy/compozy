package core

import (
	"context"

	"net/http"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"

	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) soulActorForRequest() soul.AuthoringIdentity {
	return soul.AuthoringIdentity{
		Kind: "user",
		Ref:  h.transportName(),
	}
}

func (h *BaseHandlers) soulOriginForRequest(c *gin.Context) soul.AuthoringIdentity {
	action := ""
	if c != nil && c.Request != nil {
		action = c.Request.Method + " " + c.FullPath()
	}
	return soul.AuthoringIdentity{
		Kind: h.transportName(),
		Ref:  strings.TrimSpace(action),
	}
}

func (h *BaseHandlers) heartbeatActorForRequest() heartbeat.AuthoringIdentity {
	return heartbeat.AuthoringIdentity{
		Kind: string(heartbeat.ActorKindUser),
		Ref:  h.transportName(),
	}
}

func (h *BaseHandlers) rejectHeartbeatIfMatch(c *gin.Context) bool {
	return h.rejectExpectedDigestHeader(c, "heartbeat_if_match_header_unsupported", StatusForHeartbeatError)
}

func (h *BaseHandlers) rejectSoulIfMatch(c *gin.Context) bool {
	return h.rejectExpectedDigestHeader(c, "soul_if_match_header_unsupported", StatusForSoulError)
}

func (h *BaseHandlers) rejectExpectedDigestHeader(
	c *gin.Context,
	code string,
	statusFor func(error) int,
) bool {
	if strings.TrimSpace(c.GetHeader("If-Match")) == "" {
		return false
	}
	err := newAuthoredValidationError(code + ": use expected_digest in request body")
	h.respondError(c, statusFor(err), err)
	return true
}

func (h *BaseHandlers) sessionHealthPayloadForRoute(c *gin.Context) (contract.SessionHealthPayload, bool) {
	if h.SessionHealth == nil {
		h.respondError(c, StatusForHeartbeatError(errSessionHealthMissing), errSessionHealthMissing)
		return contract.SessionHealthPayload{}, false
	}
	scope, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return contract.SessionHealthPayload{}, false
	}
	health, err := h.SessionHealth.GetSessionHealth(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return contract.SessionHealthPayload{}, false
	}
	if strings.TrimSpace(health.WorkspaceID) != scope.SessionWorkspaceID() {
		h.respondError(c, http.StatusNotFound, errWorkspaceScopedResourceNotFound)
		return contract.SessionHealthPayload{}, false
	}
	payload, err := contract.SessionHealthPayloadFromDomain(health)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return contract.SessionHealthPayload{}, false
	}
	return payload, true
}

func (h *BaseHandlers) sessionPayloadWithOptionalHealth(
	ctx context.Context,
	info *session.Info,
	includeHealth bool,
) (contract.SessionPayload, error) {
	payload := SessionPayloadFromInfo(info)
	if !includeHealth {
		return payload, nil
	}
	if h.SessionHealth == nil {
		return contract.SessionPayload{}, errSessionHealthMissing
	}
	health, err := h.SessionHealth.GetSessionHealth(ctx, payload.ID)
	if err != nil {
		return contract.SessionPayload{}, err
	}
	payload.Badge = session.BadgeForHealth(info, health)
	converted, err := contract.SessionHealthPayloadFromDomain(health)
	if err != nil {
		return contract.SessionPayload{}, err
	}
	payload.Health = &converted
	return payload, nil
}

func (h *BaseHandlers) heartbeatWakeEvents(
	ctx context.Context,
	target authoredAgentTarget,
	sessionID string,
) ([]contract.HeartbeatWakeEventPayload, error) {
	if h.HeartbeatWakeEvents == nil {
		return nil, nil
	}
	events, err := h.HeartbeatWakeEvents.ListHeartbeatWakeEvents(ctx, heartbeat.WakeEventListQuery{
		WorkspaceID: target.storageWorkspaceID(),
		AgentName:   target.agentName,
		SessionID:   strings.TrimSpace(sessionID),
		Limit:       defaultWakeEventInspectLimit,
	})
	if err != nil {
		return nil, err
	}
	payload := make([]contract.HeartbeatWakeEventPayload, 0, len(events))
	for _, event := range events {
		converted, convertErr := contract.HeartbeatWakeEventPayloadFromDomain(event)
		if convertErr != nil {
			return nil, convertErr
		}
		payload = append(payload, converted)
	}
	return payload, nil
}
