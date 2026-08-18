package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) loopRequests(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRequestsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeLoopWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().ListLoopRequests(ctx, workspaceID, core.LoopRequestListQuery{
		RunID: input.RunID, State: input.State, Cursor: input.Cursor, Limit: input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(
		response,
		fmt.Sprintf("%d requests; %d pending", len(response.Items), response.Aggregates.Pending),
	)
}

func (n *daemonNativeTools) loopRequest(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRequestInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := n.nativeLoopWorkspaceAndRunID(ctx, req.ToolID, input.WorkspaceID, input.RunID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().GetLoopRequest(
		ctx, workspaceID, runID, input.Generation, input.NodeID, input.ItemIndex,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("request %s is %s", response.NodeID, response.State))
}

func (n *daemonNativeTools) loopRespond(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRespondInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := n.nativeLoopWorkspaceAndRunID(ctx, req.ToolID, input.WorkspaceID, input.RunID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	response, err := n.loopService().RespondLoopRequest(ctx, workspaceID, runID, input.NodeID,
		contract.RespondLoopRequest{Generation: input.Generation, ItemIndex: input.ItemIndex, Decision: input.Decision,
			Payload: input.Payload, Note: input.Note}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("request %s answered", response.NodeID))
}

func (n *daemonNativeTools) loopNodeAmend(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopNodeAmendInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := n.nativeLoopWorkspaceAndRunID(ctx, req.ToolID, input.WorkspaceID, input.RunID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	response, err := n.loopService().AmendLoopNode(ctx, workspaceID, runID, input.NodeID,
		contract.LoopNodeAmendRequest{
			ItemIndex: input.ItemIndex, Payload: input.Payload, Reason: input.Reason,
		}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("loop node %s amended", response.Amendment.NodeID))
}
