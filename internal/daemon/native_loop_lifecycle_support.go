package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeLoopRunLifecycleMutation func(
	context.Context,
	string,
	string,
	taskpkg.ActorContext,
) (contract.LoopMutationResponse, error)

type nativeLoopNodeLifecycleMutation func(
	context.Context,
	string,
	string,
	string,
	contract.LoopNodeMutationRequest,
	taskpkg.ActorContext,
) (contract.LoopMutationResponse, error)

func (n *daemonNativeTools) loopLifecycleRunMutation(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	mutate nativeLoopRunLifecycleMutation,
	preview string,
) (toolspkg.ToolResult, error) {
	var input nativeLoopRunIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, err := n.nativeLoopWorkspaceAndRunID(
		ctx, req.ToolID, input.WorkspaceID, input.RunID, scope,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := mutate(ctx, workspaceID, runID, actor)
	return nativeLoopMutationResult(req.ToolID, response, err, fmt.Sprintf("loop run %s %s", runID, preview))
}

func (n *daemonNativeTools) loopNodeSimpleMutation(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	mutate nativeLoopNodeLifecycleMutation,
	preview string,
) (toolspkg.ToolResult, error) {
	var input nativeLoopNodeMutationInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, nodeID, actor, err := n.nativeLoopNodeMutationContext(
		ctx, scope, req, input.WorkspaceID, input.RunID, input.NodeID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := mutate(
		ctx,
		workspaceID,
		runID,
		nodeID,
		contract.LoopNodeMutationRequest{Reason: input.Reason, ItemIndex: input.ItemIndex},
		actor,
	)
	return nativeLoopMutationResult(
		req.ToolID, response, err, fmt.Sprintf("loop node %s %s", nodeID, preview),
	)
}

func (n *daemonNativeTools) nativeLoopNodeMutationContext(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	workspace string,
	runID string,
	nodeID string,
) (string, string, string, taskpkg.ActorContext, error) {
	workspaceID, normalizedRunID, err := n.nativeLoopWorkspaceAndRunID(
		ctx, req.ToolID, workspace, runID, scope,
	)
	if err != nil {
		return "", "", "", taskpkg.ActorContext{}, err
	}
	normalizedNodeID, err := requiredNativeString(req.ToolID, "node_id", nodeID)
	if err != nil {
		return "", "", "", taskpkg.ActorContext{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return "", "", "", taskpkg.ActorContext{}, err
	}
	return workspaceID, normalizedRunID, normalizedNodeID, actor, nil
}

func nativeLoopMutationResult(
	id toolspkg.ToolID,
	response contract.LoopMutationResponse,
	err error,
	preview string,
) (toolspkg.ToolResult, error) {
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(id, err)
	}
	return structuredResult(response, preview)
}
