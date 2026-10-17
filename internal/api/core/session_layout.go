package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// SessionLayoutManager owns workspace session folders and manual session order.
type SessionLayoutManager interface {
	ListSessionFolders(ctx context.Context, scope store.SessionLayoutScope) ([]store.SessionFolder, error)
	CreateSessionFolder(ctx context.Context, scope store.SessionLayoutScope, name string) (store.SessionFolder, error)
	RenameSessionFolder(
		ctx context.Context,
		scope store.SessionLayoutScope,
		folderID string,
		name string,
	) (store.SessionFolder, error)
	DeleteSessionFolder(ctx context.Context, scope store.SessionLayoutScope, folderID string) error
	ReorderSessionFolders(
		ctx context.Context,
		scope store.SessionLayoutScope,
		folderIDs []string,
	) ([]store.SessionFolder, error)
	MoveSession(ctx context.Context, scope store.SessionLayoutScope, sessionID string, folderID string) error
	ReorderSessions(ctx context.Context, scope store.SessionLayoutScope, folderID string, sessionIDs []string) error
}

// ListSessionFolders returns the caller profile's folders in one workspace.
func (h *BaseHandlers) ListSessionFolders(c *gin.Context) {
	manager, scope, ok := h.sessionLayoutRequest(c)
	if !ok {
		return
	}
	h.respondSessionFolders(c, manager, scope)
}

// CreateSessionFolder adds a folder at the top of the folder order.
func (h *BaseHandlers) CreateSessionFolder(c *gin.Context) {
	manager, scope, ok := h.sessionLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.CreateSessionFolderRequest
	if !h.decodeSessionLayoutBody(c, "session folder create", &req) {
		return
	}
	folder, err := manager.CreateSessionFolder(c.Request.Context(), scope, req.Name)
	if err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.SessionFolderResponse{Folder: SessionFolderPayload(folder)})
}

// RenameSessionFolder changes one folder's display name.
func (h *BaseHandlers) RenameSessionFolder(c *gin.Context) {
	manager, scope, ok := h.sessionLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.RenameSessionFolderRequest
	if !h.decodeSessionLayoutBody(c, "session folder rename", &req) {
		return
	}
	folder, err := manager.RenameSessionFolder(c.Request.Context(), scope, c.Param("folder_id"), req.Name)
	if err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionFolderResponse{Folder: SessionFolderPayload(folder)})
}

// DeleteSessionFolder removes a folder; its sessions return to the workspace root.
func (h *BaseHandlers) DeleteSessionFolder(c *gin.Context) {
	manager, scope, ok := h.sessionLayoutRequest(c)
	if !ok {
		return
	}
	if err := manager.DeleteSessionFolder(c.Request.Context(), scope, c.Param("folder_id")); err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return
	}
	h.respondSessionFolders(c, manager, scope)
}

// ReorderSessionFolders applies a complete folder order.
func (h *BaseHandlers) ReorderSessionFolders(c *gin.Context) {
	manager, scope, ok := h.sessionLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.ReorderSessionFoldersRequest
	if !h.decodeSessionLayoutBody(c, "session folder order", &req) {
		return
	}
	folders, err := manager.ReorderSessionFolders(c.Request.Context(), scope, req.FolderIDs)
	if err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionFoldersResponse{Folders: SessionFolderPayloads(folders)})
}

// MoveSession moves a session into a folder or back to the workspace root.
func (h *BaseHandlers) MoveSession(c *gin.Context) {
	manager, scope, ok := h.sessionLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.MoveSessionRequest
	if !h.decodeSessionLayoutBody(c, "session move", &req) {
		return
	}
	if err := manager.MoveSession(c.Request.Context(), scope, c.Param("session_id"), req.FolderID); err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return
	}
	h.respondSessionFolders(c, manager, scope)
}

// ReorderSessions pins a manual session order inside one container.
func (h *BaseHandlers) ReorderSessions(c *gin.Context) {
	manager, scope, ok := h.sessionLayoutRequest(c)
	if !ok {
		return
	}
	var req contract.ReorderSessionsRequest
	if !h.decodeSessionLayoutBody(c, "session order", &req) {
		return
	}
	if err := manager.ReorderSessions(c.Request.Context(), scope, req.FolderID, req.SessionIDs); err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return
	}
	h.respondSessionFolders(c, manager, scope)
}

// SessionFolderPayload maps one store folder to its wire shape.
func SessionFolderPayload(folder store.SessionFolder) contract.SessionFolderPayload {
	return contract.SessionFolderPayload{
		ID:           folder.ID,
		WorkspaceID:  folder.WorkspaceID,
		ProfileID:    folder.ProfileID,
		Name:         folder.Name,
		Position:     folder.Position,
		SessionCount: folder.SessionCount,
		CreatedAt:    folder.CreatedAt,
		UpdatedAt:    folder.UpdatedAt,
	}
}

// SessionFolderPayloads maps store folders to their wire shape.
func SessionFolderPayloads(folders []store.SessionFolder) []contract.SessionFolderPayload {
	payloads := make([]contract.SessionFolderPayload, 0, len(folders))
	for _, folder := range folders {
		payloads = append(payloads, SessionFolderPayload(folder))
	}
	return payloads
}

// ApplySessionPlacements decorates catalog payloads with their folder ids.
func ApplySessionPlacements(payloads []contract.SessionPayload, placements map[string]store.SessionPlacement) {
	for index := range payloads {
		if placement, ok := placements[payloads[index].ID]; ok {
			payloads[index].FolderID = placement.FolderID
		}
	}
}

// StatusForSessionLayoutError maps folder and manual-order errors to transport statuses.
func StatusForSessionLayoutError(err error) int {
	switch {
	case errors.Is(err, store.ErrSessionFolderNotFound):
		return http.StatusNotFound
	case errors.Is(err, store.ErrSessionLayoutInvalid):
		return http.StatusBadRequest
	case errors.Is(err, store.ErrSessionLayoutConflict),
		errors.Is(err, store.ErrSessionFolderLimit),
		errors.Is(err, store.ErrSessionLayoutSubagent):
		return http.StatusConflict
	case errors.Is(err, session.ErrSessionLayoutUnavailable):
		return http.StatusServiceUnavailable
	default:
		return StatusForSessionError(err)
	}
}

func (h *BaseHandlers) sessionLayoutRequest(
	c *gin.Context,
) (SessionLayoutManager, store.SessionLayoutScope, bool) {
	manager, ok := h.Sessions.(SessionLayoutManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, session.ErrSessionLayoutUnavailable)
		return nil, store.SessionLayoutScope{}, false
	}
	workspace, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return nil, store.SessionLayoutScope{}, false
	}
	selection, err := h.resolveProfileMutationSelectionForWorkspace(c, workspace.SessionWorkspaceID())
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return nil, store.SessionLayoutScope{}, false
	}
	return manager, store.SessionLayoutScope{
		ProfileID:   strings.TrimSpace(selection.Scope.ProfileID),
		WorkspaceID: workspace.SessionWorkspaceID(),
	}, true
}

func (h *BaseHandlers) decodeSessionLayoutBody(c *gin.Context, action string, target any) bool {
	if err := decodeStrictJSONBody(c, target); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode %s request: %w", h.transportName(), action, err),
		)
		return false
	}
	return true
}

func (h *BaseHandlers) respondSessionFolders(
	c *gin.Context,
	manager SessionLayoutManager,
	scope store.SessionLayoutScope,
) {
	folders, err := manager.ListSessionFolders(c.Request.Context(), scope)
	if err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SessionFoldersResponse{Folders: SessionFolderPayloads(folders)})
}
