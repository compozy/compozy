package daemon

import (
	"context"

	"fmt"

	"strings"

	core "github.com/compozy/compozy/internal/api/core"

	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (n *daemonNativeTools) workspaceList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct{}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	var workspaces []workspacepkg.Workspace
	if !scope.Operator && strings.TrimSpace(scope.WorkspaceID) != "" {
		resolved, err := n.deps.Workspaces.Resolve(ctx, scope.WorkspaceID)
		if err != nil {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
		workspaces = []workspacepkg.Workspace{resolved.Workspace}
	} else {
		var err error
		workspaces, err = n.deps.Workspaces.List(ctx)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	payload := make([]any, 0, len(workspaces))
	for _, workspace := range workspaces {
		payload = append(payload, core.WorkspacePayloadFromWorkspace(workspace))
	}
	return structuredResult(map[string]any{"workspaces": payload}, fmt.Sprintf("%d workspaces", len(payload)))
}

func (n *daemonNativeTools) workspaceInfo(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input workspaceRefInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	ref, err := requiredNativeString(req.ToolID, nativeToolsWorkspaceKey, input.Workspace)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspace, err := n.deps.Workspaces.Get(ctx, ref)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.WorkspacePayloadFromWorkspace(workspace)
	return structuredResult(map[string]any{nativeToolsWorkspaceKey: payload}, payload.ID)
}

func (n *daemonNativeTools) workspaceDescribe(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input workspaceRefInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	ref, err := requiredNativeString(req.ToolID, nativeToolsWorkspaceKey, input.Workspace)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.deps.Workspaces.Resolve(ctx, ref)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessions, err := n.deps.Sessions.ListAll(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	agents, err := n.workspaceAgents(ctx, &resolved)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{
		nativeToolsWorkspaceKey: core.WorkspacePayloadFromWorkspace(resolved.Workspace),
		nativeToolsSessionsKey:  core.SessionPayloadsForWorkspace(sessions, workspaceID),
		nativeToolsAgentsKey:    core.AgentPayloadsFromEntries(agents),
		nativeToolsSkillsKey:    core.WorkspaceSkillPayloads(resolved.Skills),
		nativeToolsProvidersKey: core.SessionProviderOptionPayloadsFromConfig(&resolved.Config),
	}, workspaceID)
}

func (n *daemonNativeTools) listLogs(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	input, query, err := decodeLogQueryInput(req, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query.WorkspaceID = workspaceID
	events, err := n.deps.Observer.QueryEvents(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := logEventPayloads(events)
	payload = limitLogPayloads(payload, input.Limit)
	return structuredResult(map[string]any{nativeToolsLogsKey: payload}, fmt.Sprintf("%d logs", len(payload)))
}

func (n *daemonNativeTools) observeMetrics(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct{}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	health, err := n.deps.Observer.Health(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := redactObserveHealthPayload(core.ObserveHealthPayloadFromHealth(&health))
	return structuredResult(map[string]any{nativeToolsHealthKey: payload}, payload.Status)
}

func (n *daemonNativeTools) observeSearch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	input, query, err := decodeObserveSearchInput(req, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query.WorkspaceID = workspaceID
	query.Limit = 0
	events, err := n.deps.Observer.QueryEvents(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := filterListLogs(logEventPayloads(events), input.Query)
	payload = limitLogPayloads(payload, input.Limit)
	return structuredResult(map[string]any{nativeToolsEventsKey: payload}, fmt.Sprintf("%d logs", len(payload)))
}
