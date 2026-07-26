package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/heartbeat"
)

func (h *HostAPIHandler) handleAgentsHeartbeatGet(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatStatus == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat status service is not configured"))
	}
	var params hostAPIAgentHeartbeatGetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	result, err := h.heartbeatStatus.Inspect(ctx, heartbeat.InspectRequest{Target: target.heartbeatAuthoringTarget()})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	snapshotID := ""
	if result.Snapshot != nil {
		snapshotID = strings.TrimSpace(result.Snapshot.ID)
	}
	return apicontract.HeartbeatPolicyPayloadFromResolved(result.AgentName, &result.Policy, snapshotID)
}

func (h *HostAPIHandler) handleAgentsHeartbeatValidate(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatAuthor == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat authoring service is not configured"))
	}
	var params hostAPIAgentHeartbeatValidateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	body := params.Body
	result, err := h.heartbeatAuthor.Validate(ctx, heartbeat.ValidateRequest{
		Target: target.heartbeatAuthoringTarget(),
		Body:   &body,
	})
	return hostAPIHeartbeatPayload(target, &result.Policy, "", err)
}

func hostAPIParamsFieldPresent(raw json.RawMessage, field string) (bool, error) {
	var fields map[string]json.RawMessage
	if err := decodeHostAPIParams(raw, &fields); err != nil {
		return false, err
	}
	_, ok := fields[field]
	return ok, nil
}

func (h *HostAPIHandler) handleAgentsHeartbeatPut(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatAuthor == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat authoring service is not configured"))
	}
	var params hostAPIAgentHeartbeatPutParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	result, err := h.heartbeatAuthor.Put(ctx, heartbeat.PutRequest{
		Target:         target.heartbeatAuthoringTarget(),
		Body:           params.Body,
		ExpectedDigest: strings.TrimSpace(params.ExpectedDigest),
		Actor:          hostAPIHeartbeatActor(ctx),
	})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	return hostAPIHeartbeatMutationResponse(&result)
}

func (h *HostAPIHandler) handleAgentsHeartbeatDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatAuthor == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat authoring service is not configured"))
	}
	var params hostAPIAgentHeartbeatDeleteParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	result, err := h.heartbeatAuthor.Delete(ctx, heartbeat.DeleteRequest{
		Target:         target.heartbeatAuthoringTarget(),
		ExpectedDigest: strings.TrimSpace(params.ExpectedDigest),
		Actor:          hostAPIHeartbeatActor(ctx),
	})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	return hostAPIHeartbeatMutationResponse(&result)
}

func (h *HostAPIHandler) handleAgentsHeartbeatHistory(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatAuthor == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat authoring service is not configured"))
	}
	var params hostAPIAgentHeartbeatHistoryParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	result, err := h.heartbeatAuthor.History(ctx, heartbeat.HistoryRequest{
		Target: target.heartbeatAuthoringTarget(),
		Limit:  params.Limit,
	})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	return hostAPIHeartbeatHistoryResponse(result)
}

func (h *HostAPIHandler) handleAgentsHeartbeatRollback(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatAuthor == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat authoring service is not configured"))
	}
	var params hostAPIAgentHeartbeatRollbackParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	result, err := h.heartbeatAuthor.Rollback(ctx, heartbeat.RollbackRequest{
		Target:         target.heartbeatAuthoringTarget(),
		RevisionID:     strings.TrimSpace(params.RevisionID),
		TargetDigest:   strings.TrimSpace(params.TargetDigest),
		ExpectedDigest: strings.TrimSpace(params.ExpectedDigest),
		Actor:          hostAPIHeartbeatActor(ctx),
	})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	return hostAPIHeartbeatMutationResponse(&result)
}

func (h *HostAPIHandler) handleAgentsHeartbeatStatus(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatStatus == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat status service is not configured"))
	}
	var params hostAPIAgentHeartbeatStatusParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID != "" {
		if _, err := h.requireHostAPISessionWorkspace(ctx, target.workspaceID, sessionID); err != nil {
			return nil, err
		}
	}
	result, err := h.heartbeatStatus.Status(ctx, heartbeat.StatusRequest{
		Target:               target.heartbeatAuthoringTarget(),
		SessionID:            sessionID,
		IncludeSessionHealth: params.IncludeSessionHealth,
	})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	response, err := apicontract.HeartbeatStatusResponseFromResult(&result)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	if params.IncludeRecentWakeEvents {
		events, err := h.hostAPIWakeEvents(ctx, target, sessionID)
		if err != nil {
			return nil, mapHostAPIHeartbeatRPCError(err)
		}
		response.WakeEvents = events
	}
	return response, nil
}

func (h *HostAPIHandler) handleAgentsHeartbeatWake(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.heartbeatWake == nil {
		return nil, unavailableRPCError(errors.New("extension: heartbeat wake service is not configured"))
	}
	var params hostAPIAgentHeartbeatWakeParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID != "" {
		if _, err := h.requireHostAPISessionWorkspace(ctx, target.workspaceID, sessionID); err != nil {
			return nil, err
		}
	}
	source := heartbeat.WakeSource(params.Source)
	if source == "" {
		source = heartbeat.WakeSourceManual
	}
	decision, err := h.heartbeatWake.Wake(ctx, heartbeat.WakeRequest{
		WorkspaceID: target.workspaceID,
		AgentName:   target.agentName,
		SessionID:   sessionID,
		Source:      source,
		DryRun:      params.DryRun,
	})
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	payload, err := apicontract.HeartbeatWakeDecisionPayloadFromDomain(decision)
	if err != nil {
		return nil, mapHostAPIHeartbeatRPCError(err)
	}
	return apicontract.HeartbeatWakeResponse{Decision: payload}, nil
}
