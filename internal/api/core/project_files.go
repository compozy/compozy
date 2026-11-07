package core

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

type FileTreeReader interface {
	List(context.Context, projectfiles.Root, string, projectfiles.ListOptions) (projectfiles.Listing, error)
	Read(context.Context, projectfiles.Root, string, projectfiles.ReadOptions) (projectfiles.Content, error)
}

func (h *BaseHandlers) ListFileEntries(c *gin.Context)        { h.listProjectFiles(c) }
func (h *BaseHandlers) ListSessionFileEntries(c *gin.Context) { h.listProjectFiles(c) }
func (h *BaseHandlers) ReadFileContent(c *gin.Context)        { h.readProjectFile(c) }
func (h *BaseHandlers) ReadSessionFileContent(c *gin.Context) { h.readProjectFile(c) }
func (h *BaseHandlers) projectFileRoot(c *gin.Context) (projectfiles.Root, bool) {
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return projectfiles.Root{}, false
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return projectfiles.Root{}, false
	}
	worktreeID := c.Query("worktree_id")
	if sessionID := c.Param("session_id"); sessionID != "" {
		info, err := h.requireSessionInWorkspace(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
		if err != nil || !readScope.Matches(info.ProfileID) {
			h.projectFileError(c, projectfiles.ErrRootNotFound)
			return projectfiles.Root{}, false
		}
		if info.WorkspaceID == "" {
			h.projectFileError(c, projectfiles.ErrRootUnavailable)
			return projectfiles.Root{}, false
		}
		worktreeID = info.WorktreeID
	}
	root := projectfiles.Root{
		Kind:        projectfiles.RootKindWorkspace,
		WorkspaceID: scope.ID,
		Name:        scope.Resolved.Name,
		AbsPath:     scope.Resolved.RootDir,
	}
	if worktreeID != "" {
		if h.Worktrees == nil {
			h.projectFileError(c, projectfiles.ErrRootNotFound)
			return projectfiles.Root{}, false
		}
		item, err := h.Worktrees.Resolve(c.Request.Context(), scope.RegistryID, worktreeID)
		if err != nil {
			if errors.Is(err, worktree.ErrNotFound) {
				h.projectFileError(c, projectfiles.ErrRootNotFound)
			} else {
				h.projectFileError(c, projectfiles.ErrRootUnavailable)
			}
			return projectfiles.Root{}, false
		}
		if !readScope.Matches(item.ProfileID) {
			h.projectFileError(c, projectfiles.ErrRootNotFound)
			return projectfiles.Root{}, false
		}
		root.Kind = projectfiles.RootKindWorktree
		root.WorktreeID = item.ID
		root.AbsPath = item.Path
		root.Branch = item.Branch
		root.BaseRef = item.BaseRef
	}
	root, err = projectfiles.CanonicalRoot(root)
	if err != nil {
		h.projectFileError(c, err)
		return projectfiles.Root{}, false
	}
	return root, true
}
func fileRootPayload(root projectfiles.Root) contract.FileRootPayload {
	return contract.FileRootPayload{
		Kind:        string(root.Kind),
		WorkspaceID: root.WorkspaceID,
		WorktreeID:  root.WorktreeID,
		Name:        root.Name,
		Branch:      root.Branch,
	}
}
func (h *BaseHandlers) listProjectFiles(c *gin.Context) {
	root, ok := h.projectFileRoot(c)
	if !ok {
		return
	}
	show := true
	if raw := c.Query("show_ignored"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			h.projectFileError(c, projectfiles.ErrInvalidPath)
			return
		}
		show = value
	}
	listing, err := h.FileTree.List(
		c.Request.Context(),
		root,
		c.Query("path"),
		projectfiles.ListOptions{ShowIgnored: show},
	)
	if err != nil {
		h.projectFileError(c, err)
		return
	}
	entries := make([]contract.FileEntryPayload, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		entries = append(
			entries,
			contract.FileEntryPayload{
				Name:            entry.Name,
				Path:            entry.Path,
				Kind:            string(entry.Kind),
				Size:            entry.Size,
				Symlink:         entry.Symlink,
				Ignored:         entry.Ignored,
				Sensitive:       entry.Sensitive,
				GitStatus:       string(entry.GitStatus),
				ContainsChanges: entry.ContainsChanges,
			},
		)
	}
	c.JSON(
		http.StatusOK,
		contract.FileEntriesResponse{
			Root:              fileRootPayload(listing.Root),
			Path:              listing.Path,
			Entries:           entries,
			Truncated:         listing.Truncated,
			StatusUnavailable: listing.StatusUnavailable,
		},
	)
}
func (h *BaseHandlers) readProjectFile(c *gin.Context) {
	root, ok := h.projectFileRoot(c)
	if !ok {
		return
	}
	reveal := false
	if raw := c.Query("reveal"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			h.projectFileError(c, projectfiles.ErrInvalidPath)
			return
		}
		reveal = value
	}
	if _, app := AppTokenFromContext(c.Request.Context()); app && reveal {
		h.respondSessionFilesError(
			c,
			http.StatusForbidden,
			"reveal_forbidden",
			errors.New("sensitive content cannot be revealed by app tokens"),
		)
		return
	}
	content, err := h.FileTree.Read(
		c.Request.Context(),
		root,
		c.Query("path"),
		projectfiles.ReadOptions{Reveal: reveal},
	)
	if err != nil {
		h.projectFileError(c, err)
		return
	}
	result := contract.FileContentResponse{
		Root:        fileRootPayload(root),
		Path:        content.Path,
		Kind:        string(content.Kind),
		Size:        content.Size,
		Language:    content.Language,
		ImageMIME:   content.ImageMIME,
		ImageBase64: base64.StdEncoding.EncodeToString(content.ImageData),
		Sensitive:   content.Sensitive,
		Truncated:   content.Truncated,
		Revision:    content.Revision,
	}
	if content.Kind == projectfiles.ContentKindText {
		result.Text = new(content.Text)
	}
	c.JSON(http.StatusOK, result)
}
func (h *BaseHandlers) projectFileError(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "files_unavailable", "file read failed"
	switch {
	case errors.Is(err, projectfiles.ErrInvalidPath):
		status, code, message = 400, "invalid_path", "invalid path"
	case errors.Is(err, projectfiles.ErrPathOutsideRoot):
		status, code, message = 403, "path_outside_root", "path is outside root"
	case errors.Is(err, projectfiles.ErrPathDenied):
		status, code, message = 403, "path_denied", "path cannot be shown"
	case errors.Is(err, projectfiles.ErrRootUnavailable):
		status, code, message = 409, "root_unavailable", "root is unavailable"
	case errors.Is(err, projectfiles.ErrRootNotFound):
		status, code, message = 404, "workspace_not_found", "root not found"
		if c.Param("session_id") != "" {
			code = sessionFilesSessionNotFoundCode
		} else if c.Query("worktree_id") != "" {
			code = "worktree_not_found"
		}
	case errors.Is(err, projectfiles.ErrNotFound):
		status, code, message = 404, "file_not_found", "file not found"
	case errors.Is(err, projectfiles.ErrNotAFile):
		status, code, message = 422, "not_a_file", "path is not a file"
	case errors.Is(err, projectfiles.ErrNotADirectory):
		status, code, message = 422, "not_a_directory", "path is not a directory"
	}
	h.respondSessionFilesError(c, status, code, errors.New(message))
}
