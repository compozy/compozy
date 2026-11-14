package core

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/gin-gonic/gin"
)

// rawFileContentSecurityPolicy keeps any document rendered from project bytes script-less,
// origin-less, and unable to load anything but its own inline styles.
const rawFileContentSecurityPolicy = "sandbox; default-src 'none'; style-src 'unsafe-inline'"

func (h *BaseHandlers) ReadFileBytes(c *gin.Context) { h.readProjectFileBytes(c, "") }
func (h *BaseHandlers) ReadWorktreeFileBytes(c *gin.Context) {
	h.readProjectFileBytes(c, c.Param("worktree_id"))
}
func (h *BaseHandlers) ReadSessionFileBytes(c *gin.Context) { h.readProjectFileBytes(c, "") }

// readProjectFileBytes streams one confined file with Range support. The file path is the route's
// trailing segments so relative assets of a previewed document resolve inside the same root.
func (h *BaseHandlers) readProjectFileBytes(c *gin.Context, worktreeID string) {
	root, ok := h.projectFileRootFor(c, worktreeID)
	if !ok {
		return
	}
	file, err := h.FileTree.Open(c.Request.Context(), root, strings.TrimPrefix(c.Param("path"), "/"))
	if err != nil {
		h.projectFileError(c, err)
		return
	}
	defer h.closeProjectFileStream(file)
	mediaType, inline := projectfiles.InlineMediaType(file.Path)
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	if named := mime.FormatMediaType(disposition, map[string]string{"filename": path.Base(file.Path)}); named != "" {
		disposition = named
	}
	headers := c.Writer.Header()
	headers.Set("Content-Type", mediaType)
	headers.Set("Content-Disposition", disposition)
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Content-Security-Policy", rawFileContentSecurityPolicy)
	headers.Set("Cross-Origin-Resource-Policy", "same-origin")
	headers.Set("Cache-Control", "private, no-cache")
	headers.Set("Referrer-Policy", "no-referrer")
	headers.Set("ETag", fmt.Sprintf(`W/"%d-%d"`, file.Size, file.ModTime.UnixNano()))
	http.ServeContent(c.Writer, c.Request, "", file.ModTime, file.Content)
}

func (h *BaseHandlers) closeProjectFileStream(file projectfiles.OpenedFile) {
	err := file.Content.Close()
	if err == nil {
		return
	}
	// The descriptor error names the host path; keep only its cause.
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		err = pathErr.Err
	}
	logger := slog.Default()
	if h.Logger != nil {
		logger = h.Logger
	}
	logger.Warn("project file stream close failed", handlersOperationKey, "readFileBytes", "error", err)
}
