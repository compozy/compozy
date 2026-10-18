package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionLayoutInput struct {
	Workspace  string   `json:"workspace,omitempty"`
	FolderID   string   `json:"folder_id,omitempty"`
	Name       string   `json:"name,omitempty"`
	SessionID  string   `json:"session_id,omitempty"`
	FolderIDs  []string `json:"folder_ids,omitempty"`
	SessionIDs []string `json:"session_ids,omitempty"`
}

type sessionLayoutCall func(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	input sessionLayoutInput,
) (toolspkg.ToolResult, error)

func (n *daemonNativeTools) sessionLayoutTool(call sessionLayoutCall) toolspkg.NativeToolFunc {
	return func(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
		var input sessionLayoutInput
		if err := decodeNativeInput(req, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		manager, ok := n.deps.Sessions.(core.SessionLayoutManager)
		if !ok {
			return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "session layout capability is unavailable")
		}
		resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
		if err != nil {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
		readScope, err := n.nativeProfileReadScope(ctx, scope)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := call(ctx, manager, store.SessionLayoutScope{
			ProfileID: readScope.ProfileID, WorkspaceID: workspaceID,
		}, input)
		if errors.Is(err, store.ErrSessionLayoutInvalid) {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
		return result, err
	}
}

func sessionFolderList(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	_ sessionLayoutInput,
) (toolspkg.ToolResult, error) {
	return sessionFoldersResult(ctx, manager, scope)
}

func sessionFolderCreate(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	input sessionLayoutInput,
) (toolspkg.ToolResult, error) {
	folder, err := manager.CreateSessionFolder(ctx, scope, input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.SessionFolderResponse{Folder: core.SessionFolderPayload(folder)}, folder.ID)
}

func sessionFolderRename(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	input sessionLayoutInput,
) (toolspkg.ToolResult, error) {
	folder, err := manager.RenameSessionFolder(ctx, scope, input.FolderID, input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.SessionFolderResponse{Folder: core.SessionFolderPayload(folder)}, folder.ID)
}

func sessionFolderDelete(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	input sessionLayoutInput,
) (toolspkg.ToolResult, error) {
	if err := manager.DeleteSessionFolder(ctx, scope, input.FolderID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return sessionFoldersResult(ctx, manager, scope)
}

func sessionFolderReorder(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	input sessionLayoutInput,
) (toolspkg.ToolResult, error) {
	if _, err := manager.ReorderSessionFolders(ctx, scope, input.FolderIDs); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return sessionFoldersResult(ctx, manager, scope)
}

func sessionMove(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	input sessionLayoutInput,
) (toolspkg.ToolResult, error) {
	if err := manager.MoveSession(ctx, scope, input.SessionID, input.FolderID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return sessionFoldersResult(ctx, manager, scope)
}

func sessionReorder(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
	input sessionLayoutInput,
) (toolspkg.ToolResult, error) {
	if err := manager.ReorderSessions(ctx, scope, input.FolderID, input.SessionIDs); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return sessionFoldersResult(ctx, manager, scope)
}

func sessionFoldersResult(
	ctx context.Context,
	manager core.SessionLayoutManager,
	scope store.SessionLayoutScope,
) (toolspkg.ToolResult, error) {
	folders, err := manager.ListSessionFolders(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		contract.SessionFoldersResponse{Folders: core.SessionFolderPayloads(folders)},
		fmt.Sprintf("%d session folders", len(folders)),
	)
}
