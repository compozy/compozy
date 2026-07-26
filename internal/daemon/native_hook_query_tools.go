package daemon

import (
	"context"

	"fmt"
	"strings"

	core "github.com/compozy/agh/internal/api/core"

	hookspkg "github.com/compozy/agh/internal/hooks"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) hooksList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input hooksListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	filter, err := hookCatalogFilter(req.ToolID, input, scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeHookCatalogFilterError(req.ToolID, err)
	}
	entries, err := n.deps.Observer.QueryHookCatalog(ctx, filter)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.HookCatalogPayloadsFromEntries(entries)
	return structuredResult(
		map[string]any{nativeConfigHookToolsHooksKey: payload},
		fmt.Sprintf("%d hooks", len(payload)),
	)
}

func (n *daemonNativeTools) hooksInfo(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input hooksInfoInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	filter, err := hookCatalogFilter(req.ToolID, input.hooksListInput, scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeHookCatalogFilterError(req.ToolID, err)
	}
	entries, err := n.deps.Observer.QueryHookCatalog(ctx, filter)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	name := strings.TrimSpace(input.Name)
	for _, entry := range core.HookCatalogPayloadsFromEntries(entries) {
		if entry.Name == name {
			return structuredResult(map[string]any{nativeConfigHookToolsHookKey: entry}, entry.Name)
		}
	}
	return toolspkg.ToolResult{}, toolspkg.NewToolError(
		toolspkg.ErrorCodeNotFound,
		req.ToolID,
		fmt.Sprintf("hook %q not found", name),
		toolspkg.ErrToolNotFound,
		toolspkg.ReasonToolUnknown,
	)
}

func (n *daemonNativeTools) hooksEvents(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input hooksEventsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	filter := hookspkg.EventFilter{SyncOnly: input.SyncOnly}
	if family := strings.TrimSpace(input.Family); family != "" {
		filter.Family = hookspkg.HookEventFamily(family)
		if err := filter.Family.Validate(); err != nil {
			return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
		}
	}
	events, err := n.deps.Observer.QueryHookEvents(ctx, filter)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.HookEventPayloadsFromDescriptors(events)
	return structuredResult(
		map[string]any{nativeConfigHookToolsEventsKey: payload},
		fmt.Sprintf("%d hook events", len(payload)),
	)
}

func (n *daemonNativeTools) hooksRuns(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input hooksRunsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err := input.query()
	if err != nil {
		return toolspkg.ToolResult{}, nativeHookValidationError(req.ToolID, err)
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, sessionWorkspaceID, query.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	runs, err := n.deps.Observer.QueryHookRuns(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.HookRunPayloadsFromRecords(runs)
	return structuredResult(
		map[string]any{nativeConfigHookToolsRunsKey: payload},
		fmt.Sprintf("%d hook runs", len(payload)),
	)
}
