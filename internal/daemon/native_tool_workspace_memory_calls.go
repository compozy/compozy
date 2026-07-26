package daemon

import (
	"context"

	"fmt"

	"time"

	core "github.com/compozy/agh/internal/api/core"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) workspaceList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct{}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaces, err := n.deps.Workspaces.List(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, err
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
	workspaceID, err := nativeResolvedNetworkWorkspaceID(&resolved)
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

func (n *daemonNativeTools) memoryShow(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryShowInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	location, err := n.resolveMemoryLocation(ctx, scope, req.ToolID, input.Filename, memoryToolSelector{
		Scope:     input.Scope,
		Workspace: input.Workspace,
		AgentName: input.AgentName,
		AgentTier: input.AgentTier,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	content, err := location.Store.Read(location.Scope, location.Filename)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	redactedContent := taskpkg.RedactClaimTokens(string(content))
	return structuredResult(map[string]any{
		"filename":              location.Filename,
		nativeToolsScopeKey:     location.Scope,
		nativeToolsWorkspaceKey: location.Workspace,
		"content":               redactedContent,
		nativeToolsRedactedKey:  redactedContent != string(content),
	}, location.Filename)
}

func (n *daemonNativeTools) memorySearch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memorySearchInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	queryText, err := requiredNativeString(req.ToolID, "query", firstNonEmpty(input.Query, input.Q))
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	location, err := n.memoryRecallStore(ctx, scope, req.ToolID, memoryToolSelector{
		Scope:     input.Scope,
		Workspace: input.Workspace,
		AgentName: input.AgentName,
		AgentTier: input.AgentTier,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	recall, err := location.Store.Recall(ctx, memcontract.Query{
		WorkspaceID: location.WorkspaceID,
		AgentName:   location.AgentName,
		QueryText:   queryText,
	}, memcontract.RecallOptions{
		TopK: input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	payload := redactMemoryPackaged(recall)
	results := nativeMemoryRecallResults(payload)
	return structuredResult(map[string]any{
		"recall":  payload,
		"results": results,
	}, fmt.Sprintf("%d memory results", len(results)))
}

func (n *daemonNativeTools) memoryNote(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryNoteInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	content, err := requiredNativeString(req.ToolID, "content", input.Content)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	location, err := n.memoryWriteStore(ctx, scope, req.ToolID, memoryToolSelector{
		Scope:     input.Scope,
		Workspace: input.Workspace,
		AgentName: input.AgentName,
		AgentTier: input.AgentTier,
	}, "")
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	actorKind, err := n.memoryCallerActorKind(ctx, scope, req)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	if err := n.denySubagentMemoryWrite(ctx, req, location, actorKind, input.Slug); err != nil {
		return toolspkg.ToolResult{}, err
	}
	filename := nativeMemoryAdHocFilename(input.Slug, content, time.Now().UTC())
	document, err := renderNativeMemoryDocument(nativeMemoryWriteDocument{
		Filename:    filename,
		Scope:       location.Scope,
		AgentName:   location.AgentName,
		AgentTier:   location.AgentTier,
		Name:        "Ad Hoc Memory Note",
		Description: nativeMemoryDescription(content),
		Type:        string(nativeMemoryTypeForScope("", location.Scope)),
		Content:     nativeMemoryTaggedContent(content, input.Tags),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	result, err := location.Store.ProposeWrite(ctx, location.Scope, filename, document, memcontract.OriginTool)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	n.recordMemoryToolWrite(scope, req, actorKind)
	return nativeMemoryDecisionResult(result)
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
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
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
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
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
