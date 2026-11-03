package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) DownloadTerminalRecording(c *gin.Context) {
	service, scope, ok := h.terminalReadService(c)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	recordingID := strings.TrimSpace(c.Param("id"))
	var ref *terminalpkg.RecordingRef
	var reader io.ReadCloser
	err := h.eachTerminalDownloadScope(c.Request.Context(), scope, func(owner store.ReadScope) error {
		var openErr error
		ref, reader, openErr = service.Journal().Recording(c.Request.Context(), workspaceID, owner, recordingID)
		return openErr
	})
	if err != nil {
		h.respondTerminalError(c, terminalDownloadError(recordingID, err))
		return
	}
	if ref == nil {
		h.respondTerminalError(c, errors.New("terminal recording store returned nil metadata"))
		return
	}
	h.streamTerminalDownload(c, reader, terminalDownload{
		contentType: "application/x-asciicast", filename: recordingID + ".cast", contentLength: ref.Bytes,
	})
}

func (h *BaseHandlers) DownloadTerminalArtifact(c *gin.Context) {
	service, scope, ok := h.terminalReadService(c)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	artifactID := strings.TrimSpace(c.Param("id"))
	var reader io.ReadCloser
	err := h.eachTerminalDownloadScope(c.Request.Context(), scope, func(owner store.ReadScope) error {
		var openErr error
		reader, openErr = service.Journal().Artifact(c.Request.Context(), workspaceID, owner, artifactID)
		return openErr
	})
	if err != nil {
		h.respondTerminalError(c, terminalDownloadError(artifactID, err))
		return
	}
	h.streamTerminalDownload(c, reader, terminalDownload{
		contentType: "application/octet-stream", filename: artifactID + ".log",
	})
}

// eachTerminalDownloadScope opens a download in the resolved scope. Some journals, such as the
// User scope's per-profile journals, cannot open an aggregate scope, so a file the aggregate
// does not find is opened from each owner profile in turn; the first owner that has it wins.
func (h *BaseHandlers) eachTerminalDownloadScope(
	ctx context.Context,
	scope store.ReadScope,
	open func(store.ReadScope) error,
) error {
	err := open(scope)
	if !scope.AllProfiles || !errors.Is(err, os.ErrNotExist) {
		return err
	}
	profileIDs, listErr := h.terminalOwnerProfileIDs(ctx)
	if listErr != nil {
		return err
	}
	for _, profileID := range profileIDs {
		ownerErr := open(store.ReadScope{ProfileID: profileID})
		if !errors.Is(ownerErr, os.ErrNotExist) {
			return ownerErr
		}
	}
	return err
}

type terminalDownload struct {
	contentType   string
	filename      string
	contentLength int64
}

func (h *BaseHandlers) streamTerminalDownload(c *gin.Context, reader io.ReadCloser, download terminalDownload) {
	if reader == nil {
		h.respondTerminalError(c, errors.New("terminal artifact store returned a nil reader"))
		return
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": download.filename})
	c.Header("Content-Type", download.contentType)
	c.Header("Content-Disposition", disposition)
	if download.contentLength > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", download.contentLength))
	}
	c.Status(http.StatusOK)
	_, copyErr := io.Copy(c.Writer, reader)
	closeErr := reader.Close()
	h.logTerminalDownloadError(errors.Join(copyErr, closeErr))
}

func terminalDownloadError(id string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return &terminalpkg.Error{
			Code: terminalpkg.ErrorCodeNotFound, Message: fmt.Sprintf("terminal artifact %s was not found", id),
			Err: terminalpkg.ErrNotFound,
		}
	}
	return err
}

func (h *BaseHandlers) logTerminalDownloadError(err error) {
	if err == nil {
		return
	}
	logger := slog.Default()
	if h != nil && h.Logger != nil {
		logger = h.Logger
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		logger.Debug("api: terminal download canceled", handlersErrorKey, err)
		return
	}
	logger.Warn("api: terminal download failed", handlersErrorKey, err)
}
