package daemon

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"

	"github.com/compozy/agh/internal/heartbeat"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) sessionStatus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, req.ToolID, sessionWorkspaceID, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

func (n *daemonNativeTools) sessionHealth(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, sessionWorkspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if n == nil || n.deps == nil || n.deps.SessionHealth == nil {
		return toolspkg.ToolResult{}, errors.New("daemon: session health service is required")
	}
	health, err := n.deps.SessionHealth.GetSessionHealth(ctx, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err := contract.SessionHealthPayloadFromDomain(health)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := contract.ValidateAuthoredContextRedacted(payload); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{nativeToolsHealthKey: payload}, string(payload.Health))
}

func (n *daemonNativeTools) agentHeartbeatStatus(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input agentHeartbeatStatusInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, err := n.authoredAgentTarget(ctx, req.ToolID, input.WorkspaceID, input.AgentName)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID != "" {
		if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, target.workspaceID, sessionID); err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	status, err := n.deps.HeartbeatStatus.Status(ctx, heartbeat.StatusRequest{
		Target:               target.heartbeatAuthoringTarget(),
		SessionID:            sessionID,
		IncludeSessionHealth: input.IncludeSessionHealth,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err := contract.HeartbeatStatusResponseFromResult(&status)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.IncludeRecentWakeEvents && n.deps.WakeEvents != nil {
		events, err := n.deps.WakeEvents.ListHeartbeatWakeEvents(ctx, heartbeat.WakeEventListQuery{
			WorkspaceID: target.workspaceID,
			AgentName:   target.agentName,
			SessionID:   sessionID,
			Limit:       defaultNativeWakeEventLimit,
		})
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		payload.WakeEvents = make([]contract.HeartbeatWakeEventPayload, 0, len(events))
		for _, event := range events {
			converted, convertErr := contract.HeartbeatWakeEventPayloadFromDomain(event)
			if convertErr != nil {
				return toolspkg.ToolResult{}, convertErr
			}
			payload.WakeEvents = append(payload.WakeEvents, converted)
		}
	}
	if err := contract.ValidateAuthoredContextRedacted(payload); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"heartbeat": payload}, string(payload.ValidationStatus))
}

func (n *daemonNativeTools) agentHeartbeatWake(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input agentHeartbeatWakeInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, err := n.authoredAgentTarget(ctx, req.ToolID, input.WorkspaceID, input.AgentName)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, target.workspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	source := heartbeat.WakeSource(strings.TrimSpace(input.Source))
	if source == "" {
		source = heartbeat.WakeSourceManual
	}
	decision, err := n.deps.HeartbeatWake.Wake(ctx, heartbeat.WakeRequest{
		WorkspaceID: target.workspaceID,
		AgentName:   target.agentName,
		SessionID:   sessionID,
		Source:      source,
		DryRun:      input.DryRun,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err := contract.HeartbeatWakeDecisionPayloadFromDomain(decision)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response := contract.HeartbeatWakeResponse{Decision: payload}
	if err := contract.ValidateAuthoredContextRedacted(response); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{"wake": response}, string(payload.Result))
}

func (n *daemonNativeTools) sessionEvents(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	input, query, err := decodeSessionEventQueryInput(req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, req.ToolID, sessionWorkspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	events, err := n.deps.Sessions.Events(ctx, input.SessionID, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := make([]any, 0, len(events))
	for _, event := range events {
		payload = append(payload, core.SessionEventPayloadFromEvent(event, info))
	}
	return structuredResult(map[string]any{nativeToolsEventsKey: payload}, fmt.Sprintf("%d events", len(payload)))
}

func (n *daemonNativeTools) sessionHistory(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	input, query, err := decodeSessionEventQueryInput(req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, req.ToolID, sessionWorkspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	history, err := n.deps.Sessions.History(ctx, input.SessionID, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := sessionHistoryPayload(history, info)
	return structuredResult(map[string]any{nativeToolsHistoryKey: payload}, fmt.Sprintf("%d turns", len(payload)))
}

func (n *daemonNativeTools) sessionDescribe(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	input, query, err := decodeSessionEventQueryInput(req)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	bound, err := n.nativeBoundSession(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	input.SessionID = nativeBoundSessionID(bound, input.SessionID)
	resolved, err := n.nativeResolvedWorkspace(
		ctx,
		req.ToolID,
		nativeBoundWorkspaceRef(bound, input.WorkspaceID),
		scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, req.ToolID, sessionWorkspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	events, err := n.deps.Sessions.Events(ctx, input.SessionID, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	history, err := n.deps.Sessions.History(ctx, input.SessionID, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	eventPayload := make([]any, 0, len(events))
	for _, event := range events {
		eventPayload = append(eventPayload, core.SessionEventPayloadFromEvent(event, info))
	}
	return structuredResult(map[string]any{
		nativeToolsSessionKey: core.SessionPayloadFromInfo(info),
		nativeToolsEventsKey:  eventPayload,
		nativeToolsHistoryKey: sessionHistoryPayload(history, info),
	}, info.ID)
}
