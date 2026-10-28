package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionWorktreeBindingInput struct {
	Workspace          string  `json:"workspace"`
	SessionID          string  `json:"session_id"`
	Worktree           string  `json:"worktree"`
	ExpectedWorktreeID *string `json:"expected_worktree_id"`
}

func (n *daemonNativeTools) sessionWorktreeBinding(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionWorktreeBindingInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, workspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionWorktreeBindingManager)
	if !ok {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "session worktree binding unavailable")
	}
	if req.ToolID == toolspkg.ToolIDSessionWorktreeBind && strings.TrimSpace(input.Worktree) == "" {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, errors.New("worktree is required"))
	}
	if input.ExpectedWorktreeID == nil {
		input.ExpectedWorktreeID = &info.WorktreeID
	}
	updated, err := manager.ChangeWorktreeBinding(ctx, info.ID, input.Worktree, input.ExpectedWorktreeID)
	if err != nil {
		switch {
		case errors.Is(err, session.ErrPromptInProgress):
			return toolspkg.ToolResult{}, toolspkg.NewToolError(
				toolspkg.ErrorCodeSessionBusy,
				req.ToolID,
				err.Error(),
				err,
			)
		case errors.Is(err, store.ErrSessionWorktreeConflict):
			return toolspkg.ToolResult{}, toolspkg.NewToolError(
				toolspkg.ErrorCodeSessionWorktreeConflict,
				req.ToolID,
				err.Error(),
				err,
			)
		}
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	payload := core.SessionPayloadFromInfo(updated)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

func (n *daemonNativeTools) sessionWorktreeBindingAvailability() toolspkg.NativeAvailabilityFunc {
	return n.dependencyAvailability(func() bool {
		_, ok := n.deps.Sessions.(core.SessionWorktreeBindingManager)
		return ok
	})
}
