package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) loopDiff(
	ctx context.Context,
	scope toolspkg.Scope,
	request toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopDiffInput
	if err := decodeNativeInput(request, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := n.nativeLoopWorkspaceAndRunID(
		ctx, request.ToolID, input.WorkspaceID, input.RunID, scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().DiffLoopRun(ctx, workspaceID, runID, looppkg.DiffQuery{
		Generation: input.Generation, AgainstGeneration: input.AgainstGeneration,
		AgainstRunID: looppkg.RunID(strings.TrimSpace(input.AgainstRunID)),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(request.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("%s diff with %d node rows", response.Kind, len(response.Nodes)))
}

func (n *daemonNativeTools) loopRerun(
	ctx context.Context,
	scope toolspkg.Scope,
	request toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRerunInput
	if err := decodeNativeInput(request, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := n.nativeLoopWorkspaceAndRunID(
		ctx, request.ToolID, input.WorkspaceID, input.RunID, scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(request.ToolID, err)
	}
	response, err := n.loopService().RerunLoopRun(ctx, workspaceID, runID, contract.RerunLoopRequest{
		FromNode: input.FromNode, ItemIndex: input.ItemIndex, Reason: input.Reason, RequestID: input.RequestID,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(request.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("rerun generation %d opened", response.Generation))
}

func (n *daemonNativeTools) loopFork(
	ctx context.Context,
	scope toolspkg.Scope,
	request toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopForkInput
	if err := decodeNativeInput(request, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := n.nativeLoopWorkspaceAndRunID(
		ctx, request.ToolID, input.WorkspaceID, input.RunID, scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(request.ToolID, err)
	}
	response, err := n.loopService().ForkLoopRun(ctx, workspaceID, runID, contract.ForkLoopRequest{
		Generation: input.Generation, Inputs: input.Inputs, Reason: input.Reason, RequestID: input.RequestID,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(request.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("fork run %s created", response.Run.ID))
}
