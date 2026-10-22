package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// requireCreateSessionFolder rejects a create request whose folder does not
// belong to the caller profile in the target workspace before any session or
// worktree side effect happens.
func (h *BaseHandlers) requireCreateSessionFolder(
	c *gin.Context,
	profileID string,
	req contract.CreateSessionRequest,
) bool {
	folderID := strings.TrimSpace(req.FolderID)
	if folderID == "" {
		return true
	}
	manager, ok := h.Sessions.(SessionLayoutManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, session.ErrSessionLayoutUnavailable)
		return false
	}
	ref := strings.TrimSpace(req.Workspace)
	if req.Scope == userScopeValue {
		ref = workspaceaccess.UserSelector
	}
	if ref == "" {
		ref = strings.TrimSpace(req.WorkspacePath)
	}
	if ref == "" {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf(
			"%w: folder_id requires a workspace", store.ErrSessionLayoutInvalid,
		))
		return false
	}
	var workspaceID string
	var err error
	if ref != workspaceaccess.UserSelector {
		workspaceID, err = h.lookupWorkspaceID(c.Request.Context(), ref)
	}
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return false
	}
	folders, err := manager.ListSessionFolders(c.Request.Context(), store.SessionLayoutScope{
		ProfileID: profileID, WorkspaceID: workspaceID,
	})
	if err != nil {
		h.respondError(c, StatusForSessionLayoutError(err), err)
		return false
	}
	if !slices.ContainsFunc(folders, func(folder store.SessionFolder) bool { return folder.ID == folderID }) {
		h.respondError(c, http.StatusNotFound, fmt.Errorf("%w: %s", store.ErrSessionFolderNotFound, folderID))
		return false
	}
	return true
}

// placeCreatedSession moves an accepted session into its requested folder. The
// session already exists, so a placement failure is reported in logs and the
// session stays in the workspace root rather than failing the create request.
func (h *BaseHandlers) placeCreatedSession(
	ctx context.Context,
	info *session.Info,
	folderID string,
) string {
	folderID = strings.TrimSpace(folderID)
	if folderID == "" || info == nil {
		return ""
	}
	manager, ok := h.Sessions.(SessionLayoutManager)
	if !ok {
		return ""
	}
	err := manager.MoveSession(ctx, store.SessionLayoutScope{
		ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID,
	}, info.ID, folderID)
	if err != nil {
		if h.Logger != nil && !errors.Is(err, context.Canceled) {
			h.Logger.Warn("place created session in folder", "session_id", info.ID, "folder_id", folderID, "error", err)
		}
		return ""
	}
	return folderID
}
