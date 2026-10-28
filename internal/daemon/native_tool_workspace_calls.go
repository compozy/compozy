package daemon

import (
	"context"
	"errors"
	"maps"

	"fmt"

	"strings"

	core "github.com/compozy/compozy/internal/api/core"

	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (n *daemonNativeTools) workspaceList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		Archived bool `json:"archived"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	var workspaces []workspacepkg.Workspace
	if !scope.Operator && strings.TrimSpace(scope.WorkspaceID) != "" {
		var err error
		workspaces, err = n.addressableWorkspaces(ctx, scope)
		if err != nil {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
	} else {
		var err error
		workspaces, err = n.deps.Workspaces.List(ctx)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	payload := make([]any, 0, len(workspaces))
	for _, workspace := range workspaces {
		if (workspace.ArchivedAt != nil) != input.Archived {
			continue
		}
		payload = append(payload, core.WorkspacePayloadFromWorkspace(workspace))
	}
	return structuredResult(map[string]any{"workspaces": payload}, fmt.Sprintf("%d workspaces", len(payload)))
}

// addressableWorkspaces returns the bound session's own workspace plus every other
// registered workspace the workspace-access policy already allows it to address.
// The catalog seam never prompts, so approve-reads lists only consented workspaces.
func (n *daemonNativeTools) addressableWorkspaces(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]workspacepkg.Workspace, error) {
	own, err := n.deps.Workspaces.Resolve(ctx, scope.WorkspaceID)
	if err != nil {
		return nil, err
	}
	workspaces := []workspacepkg.Workspace{own.Workspace}
	policy := n.deps.WorkspaceAccess
	if policy == nil || strings.TrimSpace(scope.SessionID) == "" {
		return workspaces, nil
	}
	catalog, err := n.deps.Workspaces.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, candidate := range catalog {
		if candidate.ID == own.ID {
			continue
		}
		decision, err := policy.Authorize(ctx, workspaceaccess.Request{
			Actor: workspaceaccess.ActorRef{
				Kind:        workspaceaccess.ActorAgentSession,
				SessionID:   strings.TrimSpace(scope.SessionID),
				WorkspaceID: own.ID,
				AgentName:   strings.TrimSpace(scope.AgentName),
			},
			Target: workspaceaccess.ScopeTarget(candidate.ID, scope.ProfileID),
			Seam:   workspaceaccess.SeamCatalog,
		})
		if err == nil && decision.Allowed {
			workspaces = append(workspaces, candidate)
		}
	}
	return workspaces, nil
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
	if input.WorkspaceID == workspaceaccess.UserSelector {
		query.WorkspaceID = workspaceaccess.UserSelector
	}
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
	result := map[string]any{nativeToolsHealthKey: payload}
	if usage, ok := n.deps.ProviderUsage.(interface{ UsageMetrics() map[string]any }); ok {
		maps.Copy(result, usage.UsageMetrics())
	}
	return structuredResult(result, payload.Status)
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
	if input.WorkspaceID == workspaceaccess.UserSelector {
		query.WorkspaceID = workspaceaccess.UserSelector
	}
	query.Limit = 0
	events, err := n.deps.Observer.QueryEvents(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := filterListLogs(logEventPayloads(events), input.Query)
	payload = limitLogPayloads(payload, input.Limit)
	return structuredResult(map[string]any{nativeToolsEventsKey: payload}, fmt.Sprintf("%d logs", len(payload)))
}

func (n *daemonNativeTools) workspaceArchive(
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
	ws, err := n.deps.Workspaces.SetArchived(ctx, ref, req.ToolID == toolspkg.ToolIDWorkspaceArchive)
	if busy, ok := errors.AsType[*workspacepkg.BusyError](err); ok {
		result, resultErr := structuredResult(map[string]any{"blockers": busy.Blockers}, busy.Error())
		if resultErr != nil {
			return toolspkg.ToolResult{}, resultErr
		}
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeWorkspaceBusy, req.ToolID, busy.Error(),
			errors.Join(toolspkg.ErrToolConflict, err)).WithPartialResult(result)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeHTTPStatusToolError(req.ToolID, err, core.StatusForWorkspaceError(err))
	}
	return structuredResult(map[string]any{nativeToolsWorkspaceKey: core.WorkspacePayloadFromWorkspace(ws)}, ws.ID)
}
