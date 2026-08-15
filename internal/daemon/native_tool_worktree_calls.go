package daemon

import (
	"context"
	"fmt"

	core "github.com/compozy/compozy/internal/api/core"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/worktree"
)

type nativeWorktreeListInput struct {
	Workspace string `json:"workspace"`
	Refresh   bool   `json:"refresh"`
}

type nativeWorktreeRefInput struct {
	Workspace string `json:"workspace"`
	Ref       string `json:"ref"`
}

type nativeWorktreeCreateInput struct {
	Workspace      string `json:"workspace"`
	Name           string `json:"name"`
	Branch         string `json:"branch"`
	BaseRef        string `json:"base_ref"`
	ExistingBranch string `json:"existing_branch"`
	Path           string `json:"path"`
}

type nativeWorktreeRemoveInput struct {
	Workspace string `json:"workspace"`
	Ref       string `json:"ref"`
	Force     bool   `json:"force"`
}

func (n *daemonNativeTools) worktreeList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeWorktreeListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	listing, err := n.deps.Worktrees.ListDetails(ctx, workspaceID, input.Refresh)
	if err != nil {
		return toolspkg.ToolResult{}, nativeWorktreeToolError(req.ToolID, err)
	}
	payload := core.WorktreesPayloadFromListing(listing)
	return structuredResult(payload, fmt.Sprintf("%d worktrees", len(payload.Worktrees)))
}

func (n *daemonNativeTools) worktreeInspect(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeWorktreeRefInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, ref, err := n.nativeWorktreeTarget(ctx, scope, req.ToolID, input.Workspace, input.Ref)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	inspection, err := n.deps.Worktrees.Inspect(ctx, workspaceID, ref)
	if err != nil {
		return toolspkg.ToolResult{}, nativeWorktreeToolError(req.ToolID, err)
	}
	payload := core.WorktreeInspectionPayload(*inspection)
	return structuredResult(payload, inspection.Worktree.ID)
}

func (n *daemonNativeTools) worktreeCreate(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeWorktreeCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	item, err := n.deps.Worktrees.CreateAccepted(ctx, workspaceID, worktree.CreateOptions{
		Name: input.Name, Branch: input.Branch, BaseRef: input.BaseRef,
		ExistingBranch: input.ExistingBranch, Path: input.Path, Origin: worktree.OriginManual,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeWorktreeToolError(req.ToolID, err)
	}
	payload := core.WorktreePayloadFromInspection(worktree.Inspection{
		Worktree: *item, AgentActivity: worktree.AgentActivityIdle,
	})
	return structuredResult(map[string]any{"worktree": payload}, item.ID)
}

func (n *daemonNativeTools) worktreeRemove(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeWorktreeRemoveInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, ref, err := n.nativeWorktreeTarget(ctx, scope, req.ToolID, input.Workspace, input.Ref)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	inspection, err := n.deps.Worktrees.Inspect(ctx, workspaceID, ref)
	if err != nil {
		return toolspkg.ToolResult{}, nativeWorktreeToolError(req.ToolID, err)
	}
	worktreeID := inspection.Worktree.ID
	refusal, err := n.deps.Worktrees.Remove(ctx, workspaceID, worktreeID, input.Force)
	if refusal != nil {
		partial, marshalErr := structuredResult(core.WorktreeRemovalRefusalPayload(*refusal), refusal.Code)
		if marshalErr != nil {
			return toolspkg.ToolResult{}, marshalErr
		}
		return toolspkg.ToolResult{}, nativeWorktreeToolError(req.ToolID, err).WithPartialResult(partial)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeWorktreeToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{"worktree_id": worktreeID, "state": string(worktree.StateRemoved)},
		worktreeID,
	)
}

func (n *daemonNativeTools) nativeWorktreeTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	workspace string,
	ref string,
) (string, string, error) {
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, id, workspace, scope)
	if err != nil {
		return "", "", err
	}
	worktreeRef, err := requiredNativeString(id, "ref", ref)
	if err != nil {
		return "", "", err
	}
	return workspaceID, worktreeRef, nil
}

func nativeWorktreeToolError(id toolspkg.ToolID, err error) *toolspkg.ToolError {
	mapped := nativeHTTPStatusToolError(id, err, core.StatusForWorktreeError(err))
	toolErr, ok := mapped.(*toolspkg.ToolError)
	if ok {
		return toolErr
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeBackendFailed, id, err.Error(), err, toolspkg.ReasonBackendUnhealthy,
	)
}
