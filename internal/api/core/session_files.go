package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspacefiles"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/gin-gonic/gin"
)

const (
	defaultSessionFilesLimit = contract.SessionFilesDefaultLimit
	maxSessionFilesLimit     = contract.SessionFilesMaxLimit

	sessionFilesInvalidRequestCode  = "invalid_request"
	sessionFilesSessionNotFoundCode = "session_not_found"
	sessionDirectoryUnavailableCode = "session_directory_unavailable"
)

var (
	errSessionDirectoryUnavailable = errors.New("session working directory no longer exists")
	errSessionFilesSearchFailed    = errors.New("session file search failed")
)

// SessionFileSearcher ranks files and folders under one directory root.
type SessionFileSearcher interface {
	Search(ctx context.Context, root string, query string, limit int) (workspacefiles.Result, error)
}

// SearchSessionFiles lists ranked files and folders under one session's working directory.
func (h *BaseHandlers) SearchSessionFiles(c *gin.Context) {
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	notFound := fmt.Errorf("session %s not found", sessionID)
	info, err := h.requireSessionInWorkspace(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	if err != nil {
		if status := statusForWorkspaceScopedResourceError(err); status != http.StatusNotFound {
			h.respondError(c, status, err)
			return
		}
		h.respondSessionFilesError(c, http.StatusNotFound, sessionFilesSessionNotFoundCode, notFound)
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !readScope.Matches(info.ProfileID) {
		h.respondSessionFilesError(c, http.StatusNotFound, sessionFilesSessionNotFoundCode, notFound)
		return
	}
	limit, err := parseSessionFilesLimit(c.Query("limit"))
	if err != nil {
		h.respondSessionFilesError(c, http.StatusBadRequest, sessionFilesInvalidRequestCode, err)
		return
	}
	ctx := c.Request.Context()
	root, err := h.Sessions.WorkingDir(ctx, sessionID)
	if err != nil {
		h.respondSessionFilesRootError(c, sessionID, "", StatusForSessionError(err), err)
		return
	}
	result, err := h.SessionFiles.Search(ctx, root, c.Query("q"), limit)
	if err != nil {
		h.respondSessionFilesRootError(c, sessionID, root, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, sessionFilesResponse(result))
}

func parseSessionFilesLimit(raw string) (int, error) {
	limit, err := ParseOptionalInt(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer between 1 and %d: %w", maxSessionFilesLimit, err)
	}
	if strings.TrimSpace(raw) == "" {
		return defaultSessionFilesLimit, nil
	}
	if limit < 1 || limit > maxSessionFilesLimit {
		return 0, fmt.Errorf("limit must be an integer between 1 and %d", maxSessionFilesLimit)
	}
	return limit, nil
}

// respondSessionFilesRootError maps a vanished working directory (or bound
// worktree) to 409 and every other failure to status. API responses never carry
// absolute paths (composer-mentions Safety Invariant 6), and the cause error of
// a root lookup or search embeds the absolute root, so the 409 and every 5xx
// answer with a fixed message; the 5xx cause reaches only the daemon log, with
// root redacted.
func (h *BaseHandlers) respondSessionFilesRootError(
	c *gin.Context,
	sessionID string,
	root string,
	status int,
	err error,
) {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, worktree.ErrMissing) {
		h.respondSessionFilesError(
			c,
			http.StatusConflict,
			sessionDirectoryUnavailableCode,
			errSessionDirectoryUnavailable,
		)
		return
	}
	if status >= http.StatusInternalServerError {
		h.logSessionReadError("session.files.search", sessionID, redactSessionFilesRoot(root, err))
		h.respondError(c, status, errSessionFilesSearchFailed)
		return
	}
	h.respondError(c, status, err)
}

// redactSessionFilesRoot renders err for logs with root replaced, keeping the
// cause chain so cancellation still logs at debug level.
func redactSessionFilesRoot(root string, err error) error {
	if strings.TrimSpace(root) == "" {
		return err
	}
	return redactedRootError{message: strings.ReplaceAll(err.Error(), root, "<root>"), cause: err}
}

type redactedRootError struct {
	message string
	cause   error
}

func (e redactedRootError) Error() string { return e.message }

func (e redactedRootError) Unwrap() error { return e.cause }

func (h *BaseHandlers) respondSessionFilesError(c *gin.Context, status int, code string, err error) {
	payload := ErrorPayloadForStatus(status, err, h.MaskInternalErrors)
	payload.Code = code
	c.JSON(status, payload)
}

func sessionFilesResponse(result workspacefiles.Result) contract.SessionFilesResponse {
	entries := make([]contract.SessionFileEntryPayload, 0, len(result.Entries))
	for _, entry := range result.Entries {
		entries = append(entries, contract.SessionFileEntryPayload{
			Path: entry.Path,
			Name: entry.Name,
			Kind: contract.SessionFileKind(entry.Kind),
		})
	}
	return contract.SessionFilesResponse{Entries: entries, Truncated: result.Truncated}
}
