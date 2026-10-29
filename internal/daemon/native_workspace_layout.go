package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type workspaceLayoutInput struct {
	FolderID     string   `json:"folder_id,omitempty"`
	Name         string   `json:"name,omitempty"`
	Workspace    string   `json:"workspace,omitempty"`
	FolderIDs    []string `json:"folder_ids,omitempty"`
	WorkspaceIDs []string `json:"workspace_ids,omitempty"`
}

type workspaceLayoutCall func(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	input workspaceLayoutInput,
) (toolspkg.ToolResult, error)

// workspaceLayoutTool binds one workspace layout operation. The layout spans
// every workspace of the caller profile, so it is an aggregate surface: only
// the operator or a session with the unrestricted grant may use it.
func (n *daemonNativeTools) workspaceLayoutTool(call workspaceLayoutCall) toolspkg.NativeToolFunc {
	return func(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
		var input workspaceLayoutInput
		if err := decodeNativeInput(req, &input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		manager, ok := n.deps.Sessions.(core.WorkspaceLayoutManager)
		if !ok {
			return toolspkg.ToolResult{}, nativeUnavailableError(
				req.ToolID,
				"workspace layout capability is unavailable",
			)
		}
		if err := n.requireWorkspaceLayoutAuthority(ctx, req.ToolID, scope); err != nil {
			return toolspkg.ToolResult{}, err
		}
		readScope, err := n.nativeProfileReadScope(ctx, scope)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if input, err = n.resolveWorkspaceLayoutInput(ctx, req.ToolID, scope, input); err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := call(ctx, manager, readScope.ProfileID, input)
		if errors.Is(err, store.ErrWorkspaceLayoutInvalid) {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
		return result, err
	}
}

func (n *daemonNativeTools) requireWorkspaceLayoutAuthority(
	ctx context.Context,
	id toolspkg.ToolID,
	scope toolspkg.Scope,
) error {
	if nativeWorkspaceScopeHasOperatorAuthority(scope) {
		return nil
	}
	if n.deps.Sessions != nil && strings.TrimSpace(scope.SessionID) != "" {
		info, err := n.deps.Sessions.Status(ctx, scope.SessionID)
		if err != nil {
			return err
		}
		if info != nil && info.EffectiveWorkspaceAccess == workspaceaccess.Unrestricted {
			return nil
		}
	}
	return nativeWorkspaceAccessDeniedError(id)
}

// resolveWorkspaceLayoutInput maps workspace references to registry ids;
// "@user" names the User scope, stored as the empty id. A move without a
// workspace targets the caller's own workspace.
func (n *daemonNativeTools) resolveWorkspaceLayoutInput(
	ctx context.Context,
	id toolspkg.ToolID,
	scope toolspkg.Scope,
	input workspaceLayoutInput,
) (workspaceLayoutInput, error) {
	resolve := func(ref string) (string, error) {
		ref = strings.TrimSpace(ref)
		if ref == workspaceaccess.UserSelector {
			return "", nil
		}
		if ref == "" {
			return "", nativeInputError(
				id,
				fmt.Errorf(
					"%w: workspace is required outside a workspace-bound session",
					store.ErrWorkspaceLayoutInvalid,
				),
			)
		}
		if n.deps.Workspaces == nil {
			return "", nativeUnavailableError(id, "workspace registry is unavailable")
		}
		workspace, err := n.deps.Workspaces.Get(ctx, ref)
		if err != nil {
			return "", nativeHTTPStatusToolError(id, err, core.StatusForWorkspaceError(err))
		}
		return workspace.ID, nil
	}
	var err error
	if id == toolspkg.ToolIDWorkspaceMove {
		if strings.TrimSpace(input.Workspace) == "" {
			input.Workspace = scope.WorkspaceID
		}
		if input.Workspace, err = resolve(input.Workspace); err != nil {
			return input, err
		}
	}
	if len(input.WorkspaceIDs) > 0 {
		resolved := make([]string, 0, len(input.WorkspaceIDs))
		for _, ref := range input.WorkspaceIDs {
			workspaceID, err := resolve(ref)
			if err != nil {
				return input, err
			}
			resolved = append(resolved, workspaceID)
		}
		input.WorkspaceIDs = resolved
	}
	return input, nil
}

func workspaceFolderList(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	_ workspaceLayoutInput,
) (toolspkg.ToolResult, error) {
	return workspaceLayoutResult(ctx, manager, profileID)
}

func workspaceFolderCreate(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	input workspaceLayoutInput,
) (toolspkg.ToolResult, error) {
	folder, err := manager.CreateWorkspaceFolder(ctx, profileID, input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.WorkspaceFolderResponse{Folder: core.WorkspaceFolderPayload(folder)}, folder.ID)
}

func workspaceFolderRename(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	input workspaceLayoutInput,
) (toolspkg.ToolResult, error) {
	folder, err := manager.RenameWorkspaceFolder(ctx, profileID, input.FolderID, input.Name)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(contract.WorkspaceFolderResponse{Folder: core.WorkspaceFolderPayload(folder)}, folder.ID)
}

func workspaceFolderDelete(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	input workspaceLayoutInput,
) (toolspkg.ToolResult, error) {
	if err := manager.DeleteWorkspaceFolder(ctx, profileID, input.FolderID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return workspaceLayoutResult(ctx, manager, profileID)
}

func workspaceFolderReorder(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	input workspaceLayoutInput,
) (toolspkg.ToolResult, error) {
	if err := manager.ReorderWorkspaceFolders(ctx, profileID, input.FolderIDs); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return workspaceLayoutResult(ctx, manager, profileID)
}

func workspaceMove(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	input workspaceLayoutInput,
) (toolspkg.ToolResult, error) {
	if err := manager.MoveWorkspace(ctx, profileID, input.Workspace, input.FolderID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return workspaceLayoutResult(ctx, manager, profileID)
}

func workspaceReorder(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
	input workspaceLayoutInput,
) (toolspkg.ToolResult, error) {
	if err := manager.ReorderWorkspaces(ctx, profileID, input.FolderID, input.WorkspaceIDs); err != nil {
		return toolspkg.ToolResult{}, err
	}
	return workspaceLayoutResult(ctx, manager, profileID)
}

func workspaceLayoutResult(
	ctx context.Context,
	manager core.WorkspaceLayoutManager,
	profileID string,
) (toolspkg.ToolResult, error) {
	layout, err := manager.WorkspaceLayout(ctx, profileID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		core.WorkspaceLayoutPayload(layout),
		fmt.Sprintf("%d workspace folders, %d placements", len(layout.Folders), len(layout.Placements)),
	)
}

func (n *daemonNativeTools) workspaceLayoutToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	calls := map[toolspkg.ToolID]workspaceLayoutCall{
		toolspkg.ToolIDWorkspaceFolderList:    workspaceFolderList,
		toolspkg.ToolIDWorkspaceFolderCreate:  workspaceFolderCreate,
		toolspkg.ToolIDWorkspaceFolderRename:  workspaceFolderRename,
		toolspkg.ToolIDWorkspaceFolderDelete:  workspaceFolderDelete,
		toolspkg.ToolIDWorkspaceFolderReorder: workspaceFolderReorder,
		toolspkg.ToolIDWorkspaceMove:          workspaceMove,
		toolspkg.ToolIDWorkspaceReorder:       workspaceReorder,
	}
	bindings := make(map[toolspkg.ToolID]nativeToolBinding, len(calls))
	for id, call := range calls {
		bindings[id] = nativeToolBinding{call: n.workspaceLayoutTool(call), availability: availability}
	}
	return bindings
}
