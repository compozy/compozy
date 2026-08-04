package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionRuntimeSetInput struct {
	WorkspaceID      string                                 `json:"workspace"`
	SessionID        string                                 `json:"session_id"`
	Runtime          contract.PromptRuntimeSelectionPayload `json:"runtime"`
	ExpectedRevision *int64                                 `json:"expected_revision,omitempty"`
}

type sessionRuntimeClearInput struct {
	WorkspaceID      string `json:"workspace"`
	SessionID        string `json:"session_id"`
	ExpectedRevision *int64 `json:"expected_revision,omitempty"`
}

func (n *daemonNativeTools) sessionRuntimeSet(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionRuntimeSetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, info, err := n.sessionRuntimeSelectionTarget(ctx, scope, req.ToolID, input.WorkspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	revision, err := nativeRuntimeSelectionRevision(req.ToolID, info.RuntimeSelectionRevision, input.ExpectedRevision)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	selection := contract.PromptRuntimeSelectionFromPayload(&input.Runtime)
	updated, err := manager.SetRuntimeSelection(ctx, info.ID, *selection, revision)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(updated)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

func (n *daemonNativeTools) sessionRuntimeClear(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionRuntimeClearInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, info, err := n.sessionRuntimeSelectionTarget(ctx, scope, req.ToolID, input.WorkspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	revision, err := nativeRuntimeSelectionRevision(req.ToolID, info.RuntimeSelectionRevision, input.ExpectedRevision)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	updated, err := manager.ClearRuntimeSelection(ctx, info.ID, revision)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(updated)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

func (n *daemonNativeTools) sessionRuntimeSelectionTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	workspaceID string,
	sessionID string,
) (core.SessionRuntimeSelectionManager, *session.Info, error) {
	resolved, err := n.nativeResolvedWorkspace(ctx, toolID, workspaceID, scope)
	if err != nil {
		return nil, nil, err
	}
	registryWorkspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return nil, nil, nativeNetworkInputError(toolID, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, toolID, registryWorkspaceID, sessionID)
	if err != nil {
		return nil, nil, err
	}
	manager, ok := n.deps.Sessions.(core.SessionRuntimeSelectionManager)
	if !ok {
		return nil, nil, errors.New("daemon: session runtime selection manager is required")
	}
	return manager, info, nil
}

func nativeRuntimeSelectionRevision(
	toolID toolspkg.ToolID,
	current int64,
	expected *int64,
) (int64, error) {
	if expected == nil {
		return current, nil
	}
	if *expected < 0 {
		return 0, nativeNetworkInputError(toolID, fmt.Errorf("expected_revision must not be negative"))
	}
	return *expected, nil
}
