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
	service, profileID, ok := h.terminalService(c, false)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	recordingID := strings.TrimSpace(c.Param("id"))
	ref, reader, err := service.Journal().Recording(
		c.Request.Context(), workspaceID, store.ReadScope{ProfileID: profileID}, recordingID,
	)
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
	service, profileID, ok := h.terminalService(c, false)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	artifactID := strings.TrimSpace(c.Param("id"))
	reader, err := service.Journal().Artifact(
		c.Request.Context(), workspaceID, store.ReadScope{ProfileID: profileID}, artifactID,
	)
	if err != nil {
		h.respondTerminalError(c, terminalDownloadError(artifactID, err))
		return
	}
	h.streamTerminalDownload(c, reader, terminalDownload{
		contentType: "application/octet-stream", filename: artifactID + ".log",
	})
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
	h.logTerminalDownloadError(c.Request.Context(), errors.Join(copyErr, closeErr))
}

func terminalDownloadError(id string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return &terminalpkg.Error{
			Code: "terminal_not_found", Message: fmt.Sprintf("terminal artifact %s was not found", id),
			Err: terminalpkg.ErrNotFound,
		}
	}
	return err
}

func (h *BaseHandlers) logTerminalDownloadError(ctx context.Context, err error) {
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
