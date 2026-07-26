package daemon

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) memoryAdminSessionLedger(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminSessionIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, daemonPayloadSessionIDKey, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, workspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemorySessionLedger.Get(ctx, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(response, sessionID)
}

func (n *daemonNativeTools) memoryAdminSessionReplay(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminSessionReplayInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, daemonPayloadSessionIDKey, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if _, err := n.nativeSessionInWorkspace(ctx, req.ToolID, workspaceID, sessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemorySessionLedger.Replay(ctx, sessionID, contract.MemorySessionReplayRequest{
		IncludeToolEvents: input.IncludeToolEvents,
		IncludeMemory:     input.IncludeMemory,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("%d replay events", len(response.Events)))
}

func (n *daemonNativeTools) memoryAdminSessionsPrune(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminSessionsPruneInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemorySessionLedger.Prune(ctx, contract.MemorySessionsPruneRequest{
		OlderThanHours: input.OlderThanHours,
		DryRun:         input.DryRun,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("pruned %d memory sessions", response.PrunedSessions))
}

func (n *daemonNativeTools) memoryAdminSessionsRepair(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if err := decodeNativeInput(req, &struct{}{}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.deps.MemorySessionLedger.Repair(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("repaired %d memory ledgers", response.RepairedLedgers))
}

func (n *daemonNativeTools) memoryAdminHealthWorkspaces(
	ctx context.Context,
	input memoryAdminHealthInput,
) ([]string, error) {
	workspaceRef := firstNonEmpty(input.WorkspaceID, input.Workspace)
	if strings.TrimSpace(workspaceRef) == "" {
		return nil, nil
	}
	_, workspace, err := n.memoryWorkspaceIdentity(ctx, workspaceRef)
	if err != nil {
		return nil, err
	}
	return []string{workspace}, nil
}
