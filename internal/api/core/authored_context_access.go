package core

import (
	"context"
	"log/slog"

	"net/http"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/soul"

	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) soulActorForRequest() soul.AuthoringIdentity {
	return soul.AuthoringIdentity{
		Kind: userScopeValue,
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

func (h *BaseHandlers) requireAuthoredAgentSession(
	ctx context.Context,
	target authoredAgentTarget,
	sessionID string,
) error {
	info, err := h.requireSessionInWorkspace(ctx, target.storageWorkspaceID(), sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(info.ProfileID) != target.profileID || strings.TrimSpace(info.AgentName) != target.agentName {
		return errWorkspaceScopedResourceNotFound
	}
	return nil
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

// sessionHealthPayloadForRoute checks workspace ownership before projecting session health and lifecycle state.
func (h *BaseHandlers) sessionHealthPayloadForRoute(
	c *gin.Context,
) (contract.SessionHealthPayload, session.Badge, *session.Info, bool) {
	if h.SessionHealth == nil {
		h.respondError(c, StatusForHeartbeatError(errSessionHealthMissing), errSessionHealthMissing)
		return contract.SessionHealthPayload{}, "", nil, false
	}
	sessionID, info, ok := h.routeSessionRead(c)
	if !ok {
		return contract.SessionHealthPayload{}, "", nil, false
	}
	health, err := h.SessionHealth.GetSessionHealth(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return contract.SessionHealthPayload{}, "", nil, false
	}
	if strings.TrimSpace(health.WorkspaceID) != strings.TrimSpace(info.WorkspaceID) {
		h.respondError(c, http.StatusNotFound, errWorkspaceScopedResourceNotFound)
		return contract.SessionHealthPayload{}, "", nil, false
	}
	payload, err := contract.SessionHealthPayloadFromDomain(health)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return contract.SessionHealthPayload{}, "", nil, false
	}
	payload.LifecycleState = info.State
	payload.Verified = new(info.State == session.StateStopped)
	payload.Escalated = new(info.StopEscalated)
	payload.Attention = sessionAttention(info)
	return payload, session.BadgeForHealth(info, health), info, true
}

func (h *BaseHandlers) sessionPayloadWithOptionalHealth(
	ctx context.Context,
	info *session.Info,
	includeHealth bool,
) (contract.SessionPayload, error) {
	payload, err := SessionPayloadWithQueue(ctx, h.Sessions, info)
	if err != nil {
		return contract.SessionPayload{}, err
	}
	if includeHealth {
		if h.SessionHealth == nil {
			return contract.SessionPayload{}, errSessionHealthMissing
		}
		health, err := h.SessionHealth.GetSessionHealth(ctx, payload.ID)
		if err != nil {
			return contract.SessionPayload{}, err
		}
		payload.Badge = session.BadgeForHealth(info, health)
		var prState session.PullRequestState
		if payload.PullRequest != nil {
			prState = payload.PullRequest.State
		}
		payload.BoardColumn = session.BoardColumnFor(payload.Badge, prState)
		payload.WaitingOn = sessionPayloadWaitingOn(info, payload.Badge)
		converted, err := contract.SessionHealthPayloadFromDomain(health)
		if err != nil {
			return contract.SessionPayload{}, err
		}
		payload.Health = &converted
	}
	if bindings, ok := h.Sessions.(interface {
		WorktreeBindingAvailability(context.Context, string) session.WorktreeBindingAvailability
	}); ok {
		availability := bindings.WorktreeBindingAvailability(ctx, info.ID)
		payload.WorktreeBinding = &availability
	}
	payloads := []contract.SessionPayload{payload}
	if err := h.decorateSubagentSummaries(ctx, payloads); err != nil {
		return contract.SessionPayload{}, err
	}
	h.decorateMonitorSummaries(payloads)
	h.decorateProcessSummaries(payloads)
	payloads, err = h.decorateSessionOwners(ctx, payloads)
	if err != nil {
		return contract.SessionPayload{}, err
	}
	return payloads[0], nil
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
		ProfileID:   target.profileID,
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

// SessionPayloadWithQueue adds the durable admission summary to a session detail.
func SessionPayloadWithQueue(ctx context.Context, reader interface {
	InputQueueSummary(context.Context, string) (session.InputQueueSummary, error)
}, info *session.Info) (contract.SessionPayload, error) {
	if relations, ok := reader.(interface {
		PopulateSessionRelations(context.Context, *session.Info) error
	}); ok {
		if err := relations.PopulateSessionRelations(ctx, info); err != nil {
			slog.WarnContext(ctx, "session detail relations unavailable", "session_id", info.ID, "error", err)
		}
	}
	payload := SessionPayloadFromInfo(info)
	queue, err := reader.InputQueueSummary(ctx, info.ID)
	if err != nil {
		return contract.SessionPayload{}, err
	}
	payloads := []contract.SessionPayload{payload}
	if err := DecorateSessionPullRequests(ctx, reader, []*session.Info{info}, payloads); err != nil {
		slog.WarnContext(ctx, "session detail pull request unavailable", "session_id", info.ID, "error", err)
	}
	payload = payloads[0]
	payload.Queue = &contract.SessionQueueSummaryPayload{Entries: queue.PendingInputs, Cap: queue.Cap}
	payload.ReplyWatches, err = sessionReplyWatchPayloads(ctx, reader, info.WorkspaceID, info.ID)
	if err != nil {
		return contract.SessionPayload{}, err
	}
	return payload, nil
}
