package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
)

const (
	hostAPIAuthoredContextExtensionKey = "extension"
	hostAPIAuthoredContextHostAPIKey   = "host_api"
	hostAPIAuthoredContextWorkspaceKey = "workspace"
)

const defaultHostAPIWakeEventLimit = 10

var errHostAPIAuthoredValidation = errors.New("extension: authored context validation")

type hostAPISoulAuthoringService interface {
	soul.AuthoringService
}

type hostAPISoulRefresher interface {
	RefreshSoulWithExpectedDigest(context.Context, string, string) (session.SoulRefreshResult, error)
}

type hostAPIHeartbeatAuthoringService interface {
	heartbeat.AuthoringService
}

type hostAPIHeartbeatStatusService interface {
	heartbeat.StatusService
}

type hostAPIHeartbeatWakeService interface {
	Wake(context.Context, heartbeat.WakeRequest) (heartbeat.WakeDecision, error)
}

type hostAPISessionHealthReader interface {
	GetSessionHealth(context.Context, string) (heartbeat.SessionHealth, error)
}

type hostAPIHeartbeatWakeEventReader interface {
	ListHeartbeatWakeEvents(context.Context, heartbeat.WakeEventListQuery) ([]heartbeat.WakeEvent, error)
}

type hostAPIAgentSoulGetParams = extensioncontract.AgentSoulGetParams
type hostAPIAgentSoulValidateParams = extensioncontract.AgentSoulValidateParams
type hostAPIAgentSoulPutParams = extensioncontract.AgentSoulPutParams
type hostAPIAgentSoulDeleteParams = extensioncontract.AgentSoulDeleteParams
type hostAPIAgentSoulHistoryParams = extensioncontract.AgentSoulHistoryParams
type hostAPIAgentSoulRollbackParams = extensioncontract.AgentSoulRollbackParams
type hostAPIAgentHeartbeatGetParams = extensioncontract.AgentHeartbeatGetParams
type hostAPIAgentHeartbeatValidateParams = extensioncontract.AgentHeartbeatValidateParams
type hostAPIAgentHeartbeatPutParams = extensioncontract.AgentHeartbeatPutParams
type hostAPIAgentHeartbeatDeleteParams = extensioncontract.AgentHeartbeatDeleteParams
type hostAPIAgentHeartbeatHistoryParams = extensioncontract.AgentHeartbeatHistoryParams
type hostAPIAgentHeartbeatRollbackParams = extensioncontract.AgentHeartbeatRollbackParams
type hostAPIAgentHeartbeatStatusParams = extensioncontract.AgentHeartbeatStatusParams
type hostAPIAgentHeartbeatWakeParams = extensioncontract.AgentHeartbeatWakeParams
type hostAPISessionSoulRefreshParams = extensioncontract.SessionSoulRefreshParams
type hostAPISessionHealthGetParams = extensioncontract.SessionHealthGetParams
type hostAPISessionStatusGetParams = extensioncontract.SessionStatusGetParams

type hostAPIAuthoredAgentTarget struct {
	workspaceID     string
	workspaceRoot   string
	agentName       string
	agentPath       string
	soulConfig      aghconfig.SoulConfig
	heartbeatConfig aghconfig.HeartbeatConfig
}

func (h *HostAPIHandler) handleAgentsSoulGet(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.soulAuthoring == nil {
		return nil, unavailableRPCError(errors.New("extension: soul authoring service is not configured"))
	}
	var params hostAPIAgentSoulGetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	result, err := h.soulAuthoring.Validate(ctx, soul.ValidateRequest{Target: target.soulAuthoringTarget()})
	return hostAPISoulPayload(target, &result.Soul, "", err)
}

func (h *HostAPIHandler) handleAgentsSoulValidate(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.soulAuthoring == nil {
		return nil, unavailableRPCError(errors.New("extension: soul authoring service is not configured"))
	}
	var params hostAPIAgentSoulValidateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	bodyPresent, err := hostAPIParamsFieldPresent(raw, "body")
	if err != nil {
		return nil, err
	}
	var body *string
	if bodyPresent {
		body = &params.Body
	}
	result, err := h.soulAuthoring.Validate(ctx, soul.ValidateRequest{
		Target: target.soulAuthoringTarget(),
		Body:   body,
	})
	return hostAPISoulPayload(target, &result.Soul, "", err)
}

func (h *HostAPIHandler) handleAgentsSoulPut(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.soulAuthoring == nil {
		return nil, unavailableRPCError(errors.New("extension: soul authoring service is not configured"))
	}
	var params hostAPIAgentSoulPutParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	result, err := h.soulAuthoring.Put(ctx, soul.PutRequest{
		Target:         target.soulAuthoringTarget(),
		Body:           params.Body,
		ExpectedDigest: strings.TrimSpace(params.ExpectedDigest),
		Actor:          hostAPISoulActor(ctx),
		Origin:         hostAPISoulOrigin(extensioncontract.HostAPIMethodAgentsSoulPut),
	})
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	return hostAPISoulMutationResponse(target, &result)
}

func (h *HostAPIHandler) handleAgentsSoulDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.soulAuthoring == nil {
		return nil, unavailableRPCError(errors.New("extension: soul authoring service is not configured"))
	}
	var params hostAPIAgentSoulDeleteParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	result, err := h.soulAuthoring.Delete(ctx, soul.DeleteRequest{
		Target:         target.soulAuthoringTarget(),
		ExpectedDigest: strings.TrimSpace(params.ExpectedDigest),
		Actor:          hostAPISoulActor(ctx),
		Origin:         hostAPISoulOrigin(extensioncontract.HostAPIMethodAgentsSoulDelete),
	})
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	return hostAPISoulMutationResponse(target, &result)
}

func (h *HostAPIHandler) handleAgentsSoulHistory(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.soulAuthoring == nil {
		return nil, unavailableRPCError(errors.New("extension: soul authoring service is not configured"))
	}
	var params hostAPIAgentSoulHistoryParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	result, err := h.soulAuthoring.History(ctx, soul.HistoryRequest{
		Target: target.soulAuthoringTarget(),
		Limit:  params.Limit,
	})
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	return hostAPISoulHistoryResponse(result)
}

func (h *HostAPIHandler) handleAgentsSoulRollback(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.soulAuthoring == nil {
		return nil, unavailableRPCError(errors.New("extension: soul authoring service is not configured"))
	}
	var params hostAPIAgentSoulRollbackParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	target, err := h.resolveHostAPIAuthoredAgentTarget(ctx, params.WorkspaceID, params.AgentName)
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	result, err := h.soulAuthoring.Rollback(ctx, soul.RollbackRequest{
		Target:         target.soulAuthoringTarget(),
		RevisionID:     strings.TrimSpace(params.RevisionID),
		ExpectedDigest: strings.TrimSpace(params.ExpectedDigest),
		Actor:          hostAPISoulActor(ctx),
		Origin:         hostAPISoulOrigin(extensioncontract.HostAPIMethodAgentsSoulRollback),
	})
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	return hostAPISoulMutationResponse(target, &result)
}

func (h *HostAPIHandler) handleSessionsSoulRefresh(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.soulRefresher == nil {
		return nil, unavailableRPCError(errors.New("extension: soul refresh service is not configured"))
	}
	var params hostAPISessionSoulRefreshParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(params.SessionID)
	if sessionID == "" {
		return nil, invalidParamsRPCError(errors.New("session_id is required"))
	}
	if _, err := h.requireHostAPISessionWorkspace(ctx, params.WorkspaceID, sessionID); err != nil {
		return nil, err
	}
	result, err := h.soulRefresher.RefreshSoulWithExpectedDigest(
		ctx,
		sessionID,
		strings.TrimSpace(params.ExpectedDigest),
	)
	if err != nil {
		return nil, mapHostAPISoulRPCError(err)
	}
	return hostAPIAgentSoulPayloadFromRefreshResult(result)
}
