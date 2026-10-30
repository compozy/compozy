package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

const workspaceLayoutSurface = "workspace layout"

// WorkspaceLayoutManager owns one profile's root workspace folders and manual workspace order.
type WorkspaceLayoutManager interface {
	WorkspaceLayout(ctx context.Context, profileID string) (store.WorkspaceLayout, error)
	CreateWorkspaceFolder(ctx context.Context, profileID string, name string) (store.WorkspaceFolder, error)
	RenameWorkspaceFolder(
		ctx context.Context,
		profileID string,
		folderID string,
		name string,
	) (store.WorkspaceFolder, error)
	DeleteWorkspaceFolder(ctx context.Context, profileID string, folderID string) error
	ReorderWorkspaceFolders(ctx context.Context, profileID string, folderIDs []string) error
	MoveWorkspace(ctx context.Context, profileID string, workspaceID string, folderID string) error
	ReorderWorkspaces(ctx context.Context, profileID string, folderID string, workspaceIDs []string) error
}

// GetWorkspaceLayout returns the caller profile's workspace folders and placements.
func (h *BaseHandlers) GetWorkspaceLayout(c *gin.Context) {
	manager, profileID, ok := h.workspaceLayoutRequest(c)
	if !ok {
		return
	}
	h.respondWorkspaceLayout(c, manager, profileID)
}

// CreateWorkspaceFolder adds a root folder at the top of the folder order.
func (h *BaseHandlers) CreateWorkspaceFolder(c *gin.Context) {
	manager, profileID, ok := h.workspaceLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.CreateWorkspaceFolderRequest
	if !h.decodeSessionLayoutBody(c, "workspace folder create", &req) {
		return
	}
	folder, err := manager.CreateWorkspaceFolder(c.Request.Context(), profileID, req.Name)
	if err != nil {
		h.respondError(c, StatusForWorkspaceLayoutError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.WorkspaceFolderResponse{Folder: WorkspaceFolderPayload(folder)})
}

// RenameWorkspaceFolder changes one folder's display name.
func (h *BaseHandlers) RenameWorkspaceFolder(c *gin.Context) {
	manager, profileID, ok := h.workspaceLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.RenameWorkspaceFolderRequest
	if !h.decodeSessionLayoutBody(c, "workspace folder rename", &req) {
		return
	}
	folder, err := manager.RenameWorkspaceFolder(c.Request.Context(), profileID, c.Param("folder_id"), req.Name)
	if err != nil {
		h.respondError(c, StatusForWorkspaceLayoutError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.WorkspaceFolderResponse{Folder: WorkspaceFolderPayload(folder)})
}

// DeleteWorkspaceFolder removes a folder; its workspaces return to the root.
func (h *BaseHandlers) DeleteWorkspaceFolder(c *gin.Context) {
	manager, profileID, ok := h.workspaceLayoutRequest(c)
	if !ok {
		return
	}
	if err := manager.DeleteWorkspaceFolder(c.Request.Context(), profileID, c.Param("folder_id")); err != nil {
		h.respondError(c, StatusForWorkspaceLayoutError(err), err)
		return
	}
	h.respondWorkspaceLayout(c, manager, profileID)
}

// ReorderWorkspaceFolders applies a complete workspace folder order.
func (h *BaseHandlers) ReorderWorkspaceFolders(c *gin.Context) {
	manager, profileID, ok := h.workspaceLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.ReorderWorkspaceFoldersRequest
	if !h.decodeSessionLayoutBody(c, "workspace folder order", &req) {
		return
	}
	if err := manager.ReorderWorkspaceFolders(c.Request.Context(), profileID, req.FolderIDs); err != nil {
		h.respondError(c, StatusForWorkspaceLayoutError(err), err)
		return
	}
	h.respondWorkspaceLayout(c, manager, profileID)
}

// MoveWorkspace moves a workspace into a folder or back to the root.
func (h *BaseHandlers) MoveWorkspace(c *gin.Context) {
	manager, profileID, ok := h.workspaceLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.MoveWorkspaceRequest
	if !h.decodeSessionLayoutBody(c, "workspace move", &req) {
		return
	}
	workspaceID, ok := h.resolveWorkspaceLayoutTarget(c, req.WorkspaceID)
	if !ok {
		return
	}
	if err := manager.MoveWorkspace(c.Request.Context(), profileID, workspaceID, req.FolderID); err != nil {
		h.respondError(c, StatusForWorkspaceLayoutError(err), err)
		return
	}
	h.respondWorkspaceLayout(c, manager, profileID)
}

// ReorderWorkspaces pins a manual workspace order inside one container.
func (h *BaseHandlers) ReorderWorkspaces(c *gin.Context) {
	manager, profileID, ok := h.workspaceLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.ReorderWorkspacesRequest
	if !h.decodeSessionLayoutBody(c, "workspace order", &req) {
		return
	}
	if len(req.WorkspaceIDs) > store.MaxWorkspaceLayoutOrder {
		h.respondError(c, http.StatusBadRequest, store.ErrWorkspaceLayoutInvalid)
		return
	}
	workspaceIDs := make([]string, 0, len(req.WorkspaceIDs))
	for _, ref := range req.WorkspaceIDs {
		workspaceID, ok := h.resolveWorkspaceLayoutTarget(c, ref)
		if !ok {
			return
		}
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	err := manager.ReorderWorkspaces(c.Request.Context(), profileID, req.FolderID, workspaceIDs)
	if err != nil {
		h.respondError(c, StatusForWorkspaceLayoutError(err), err)
		return
	}
	h.respondWorkspaceLayout(c, manager, profileID)
}

// WorkspaceFolderPayload maps one store workspace folder to its wire shape.
func WorkspaceFolderPayload(folder store.WorkspaceFolder) contract.WorkspaceFolderPayload {
	return contract.WorkspaceFolderPayload{
		ID:             folder.ID,
		ProfileID:      folder.ProfileID,
		Name:           folder.Name,
		Position:       folder.Position,
		WorkspaceCount: folder.WorkspaceCount,
		CreatedAt:      folder.CreatedAt,
		UpdatedAt:      folder.UpdatedAt,
	}
}

// WorkspaceLayoutPayload maps a store workspace layout to its wire shape.
func WorkspaceLayoutPayload(layout store.WorkspaceLayout) contract.WorkspaceLayoutResponse {
	response := contract.WorkspaceLayoutResponse{
		Folders:    make([]contract.WorkspaceFolderPayload, 0, len(layout.Folders)),
		Placements: make([]contract.WorkspacePlacementPayload, 0, len(layout.Placements)),
	}
	for _, folder := range layout.Folders {
		response.Folders = append(response.Folders, WorkspaceFolderPayload(folder))
	}
	for _, placement := range layout.Placements {
		workspaceID := placement.WorkspaceID
		if workspaceID == "" {
			workspaceID = workspaceaccess.UserSelector
		}
		response.Placements = append(response.Placements, contract.WorkspacePlacementPayload{
			WorkspaceID: workspaceID,
			FolderID:    placement.FolderID,
			Position:    placement.Position,
		})
	}
	return response
}

// StatusForWorkspaceLayoutError maps workspace folder and order errors to transport statuses.
func StatusForWorkspaceLayoutError(err error) int {
	switch {
	case errors.Is(err, store.ErrWorkspaceFolderNotFound),
		errors.Is(err, store.ErrWorkspaceLayoutTargetNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrWorkspaceLayoutInvalid):
		return http.StatusBadRequest
	case errors.Is(err, store.ErrWorkspaceLayoutConflict),
		errors.Is(err, store.ErrWorkspaceFolderLimit):
		return http.StatusConflict
	case errors.Is(err, session.ErrSessionLayoutUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// workspaceLayoutRequest authorizes the cross-workspace layout surface and
// resolves the single profile that owns it.
func (h *BaseHandlers) workspaceLayoutRequest(c *gin.Context) (WorkspaceLayoutManager, string, bool) {
	if !h.requireOperatorOrGrantedAgent(c, workspaceLayoutSurface) {
		return nil, "", false
	}
	manager, ok := h.Sessions.(WorkspaceLayoutManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, session.ErrSessionLayoutUnavailable)
		return nil, "", false
	}
	selection, err := h.resolveProfileReadSelection(c)
	if err == nil && selection.Scope.AllProfiles {
		err = &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "the workspace layout belongs to one profile",
			Action:  "choose one profile instead of all_profiles",
			Cause:   profilepkg.ErrInvalidInput,
		}
	}
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return nil, "", false
	}
	return manager, strings.TrimSpace(selection.Scope.ProfileID), true
}

// resolveWorkspaceLayoutTarget maps a workspace reference to its stored id;
// "@user" names the User scope, stored as the empty id.
func (h *BaseHandlers) resolveWorkspaceLayoutTarget(c *gin.Context, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == workspaceaccess.UserSelector {
		return "", true
	}
	if ref == "" {
		h.respondError(c, http.StatusBadRequest, errors.Join(
			store.ErrWorkspaceLayoutInvalid, errors.New("workspace_id is required"),
		))
		return "", false
	}
	workspaceID, err := h.lookupWorkspaceID(c.Request.Context(), ref)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return "", false
	}
	return workspaceID, true
}

func (h *BaseHandlers) respondWorkspaceLayout(c *gin.Context, manager WorkspaceLayoutManager, profileID string) {
	layout, err := manager.WorkspaceLayout(c.Request.Context(), profileID)
	if err != nil {
		h.respondError(c, StatusForWorkspaceLayoutError(err), err)
		return
	}
	c.JSON(http.StatusOK, WorkspaceLayoutPayload(layout))
}
