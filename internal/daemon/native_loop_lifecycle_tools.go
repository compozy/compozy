package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeLoopNodesInput struct {
	WorkspaceID string `json:"workspace,omitempty"`
	State       string `json:"state"`
	LoopName    string `json:"loop_name,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type nativeLoopNodeMutationInput struct {
	WorkspaceID string `json:"workspace,omitempty"`
	RunID       string `json:"run_id"`
	NodeID      string `json:"node_id"`
	ItemIndex   *int   `json:"item_index,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type nativeLoopNodePauseInput struct {
	WorkspaceID string `json:"workspace,omitempty"`
	RunID       string `json:"run_id"`
	NodeID      string `json:"node_id"`
	ItemIndex   *int   `json:"item_index,omitempty"`
	Mode        string `json:"mode"`
	Reason      string `json:"reason,omitempty"`
}

type nativeLoopNodeResumeInput struct {
	WorkspaceID string          `json:"workspace,omitempty"`
	RunID       string          `json:"run_id"`
	NodeID      string          `json:"node_id"`
	Mode        string          `json:"mode"`
	ItemIndex   *int            `json:"item_index,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

func (n *daemonNativeTools) addLoopLifecycleBindings(
	bindings map[toolspkg.ToolID]nativeToolBinding,
	availability toolspkg.NativeAvailabilityFunc,
) {
	bindings[toolspkg.ToolIDLoopCancel] = nativeToolBinding{call: n.loopCancel, availability: availability}
	bindings[toolspkg.ToolIDLoopNodes] = nativeToolBinding{call: n.loopNodes, availability: availability}
	bindings[toolspkg.ToolIDLoopNodePause] = nativeToolBinding{call: n.loopNodePause, availability: availability}
	bindings[toolspkg.ToolIDLoopNodeResume] = nativeToolBinding{call: n.loopNodeResume, availability: availability}
	bindings[toolspkg.ToolIDLoopNodeCancel] = nativeToolBinding{call: n.loopNodeCancel, availability: availability}
	bindings[toolspkg.ToolIDLoopNodeRequeue] = nativeToolBinding{call: n.loopNodeRequeue, availability: availability}
}

func (n *daemonNativeTools) loopCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.loopLifecycleRunMutation(ctx, scope, req, n.loopService().CancelLoopRun, "canceled")
}

func (n *daemonNativeTools) loopNodes(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopNodesInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeLoopWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().ListLoopNodes(ctx, workspaceID, core.LoopNodeListQuery{
		State:    strings.TrimSpace(input.State),
		LoopName: strings.TrimSpace(input.LoopName),
		RunID:    strings.TrimSpace(input.RunID),
		Cursor:   strings.TrimSpace(input.Cursor),
		Limit:    input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeLoopToolError(req.ToolID, err)
	}
	return structuredResult(response, fmt.Sprintf("%d loop nodes", len(response.Items)))
}

func (n *daemonNativeTools) loopNodePause(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopNodePauseInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, nodeID, actor, err := n.nativeLoopNodeMutationContext(
		ctx, scope, req, input.WorkspaceID, input.RunID, input.NodeID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().PauseLoopNode(ctx, workspaceID, runID, nodeID, contract.LoopNodePauseRequest{
		Mode: input.Mode, Reason: input.Reason, ItemIndex: input.ItemIndex,
	}, actor)
	return nativeLoopMutationResult(req.ToolID, response, err, fmt.Sprintf("loop node %s paused", nodeID))
}

func (n *daemonNativeTools) loopNodeResume(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeLoopNodeResumeInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, runID, nodeID, actor, err := n.nativeLoopNodeMutationContext(
		ctx, scope, req, input.WorkspaceID, input.RunID, input.NodeID,
	)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, err := n.loopService().ResumeLoopNode(ctx, workspaceID, runID, nodeID, contract.LoopNodeResumeRequest{
		Mode: input.Mode, ItemIndex: input.ItemIndex, Payload: input.Payload,
	}, actor)
	return nativeLoopMutationResult(req.ToolID, response, err, fmt.Sprintf("loop node %s resumed", nodeID))
}

func (n *daemonNativeTools) loopNodeCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.loopNodeSimpleMutation(ctx, scope, req, n.loopService().CancelLoopNode, "canceled")
}

func (n *daemonNativeTools) loopNodeRequeue(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.loopNodeSimpleMutation(ctx, scope, req, n.loopService().RequeueLoopNode, "requeued")
}
