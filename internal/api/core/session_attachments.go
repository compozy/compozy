package core

import (
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/gin-gonic/gin"
)

// SessionAttachmentMultipartOverheadBytes bounds multipart headers and framing around one uploaded file.
const SessionAttachmentMultipartOverheadBytes int64 = 64 << 10

var errInvalidSessionAttachmentUpload = errors.New("invalid session attachment upload")

// UploadSessionAttachment stores one multipart file in the authorized session scope.
func (h *BaseHandlers) UploadSessionAttachment(c *gin.Context) {
	scope, sessionID, ok := h.authorizedSessionAttachmentRoute(c)
	if !ok {
		return
	}
	if h.SessionAttachments == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session attachment store is not configured"))
		return
	}
	maxFileBytes := h.Config.Session.Attachments.MaxFileBytes
	if maxFileBytes <= 0 {
		h.respondError(c, http.StatusInternalServerError, errors.New("session attachment max_file_bytes is not configured"))
		return
	}
	name, data, err := readSessionAttachmentUpload(c, maxFileBytes)
	if err != nil {
		err = sessionAttachmentUploadError(err, maxFileBytes)
		h.respondError(c, statusForSessionAttachmentError(err), fmt.Errorf("read session attachment upload: %w", err))
		return
	}
	ref, err := h.SessionAttachments.Put(
		c.Request.Context(),
		scope.ID,
		sessionID,
		name,
		data,
	)
	if err != nil {
		err = sessionAttachmentUploadError(err, maxFileBytes)
		h.respondError(c, statusForSessionAttachmentError(err), fmt.Errorf("store session attachment: %w", err))
		return
	}
	c.JSON(http.StatusCreated, contract.SessionAttachmentUploadResponse{
		Attachment: contract.SessionAttachmentPayloadFromRef(ref),
	})
}

// ReadSessionAttachmentBytes streams one authorized attachment with its stored MIME type.
func (h *BaseHandlers) ReadSessionAttachmentBytes(c *gin.Context) {
	scope, sessionID, ok := h.authorizedSessionAttachmentRoute(c)
	if !ok {
		return
	}
	if h.SessionAttachments == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session attachment store is not configured"))
		return
	}
	id, err := sessionAttachmentID(c)
	if err != nil {
		h.respondError(c, statusForSessionAttachmentError(err), err)
		return
	}
	reader, ref, err := h.SessionAttachments.Open(c.Request.Context(), scope.ID, sessionID, id)
	if err != nil {
		h.respondError(c, statusForSessionAttachmentError(err), fmt.Errorf("open session attachment: %w", err))
		return
	}
	if reader == nil {
		h.respondError(c, http.StatusInternalServerError, errors.New("session attachment store returned a nil reader"))
		return
	}
	c.Header("Content-Type", ref.MIMEType)
	c.Header("Content-Length", fmt.Sprintf("%d", ref.Bytes))
	c.Status(http.StatusOK)
	_, copyErr := io.Copy(c.Writer, reader)
	closeErr := reader.Close()
	if readErr := errors.Join(copyErr, closeErr); readErr != nil {
		h.logSessionReadError(
			"read session attachment bytes",
			sessionID,
			readErr,
			"workspace_id",
			scope.ID,
			"attachment_id",
			id,
		)
	}
}

// DeleteSessionAttachment removes one authorized attachment.
func (h *BaseHandlers) DeleteSessionAttachment(c *gin.Context) {
	scope, sessionID, ok := h.authorizedSessionAttachmentRoute(c)
	if !ok {
		return
	}
	if h.SessionAttachments == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session attachment store is not configured"))
		return
	}
	id, err := sessionAttachmentID(c)
	if err != nil {
		h.respondError(c, statusForSessionAttachmentError(err), err)
		return
	}
	if err := h.SessionAttachments.Delete(c.Request.Context(), scope.ID, sessionID, id); err != nil {
		h.respondError(c, statusForSessionAttachmentError(err), fmt.Errorf("delete session attachment: %w", err))
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) authorizedSessionAttachmentRoute(c *gin.Context) (workspaceScope, string, bool) {
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return workspaceScope{}, "", false
	}
	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return workspaceScope{}, "", false
	}
	return scope, sessionID, true
}

func readSessionAttachmentUpload(c *gin.Context, maxFileBytes int64) (string, []byte, error) {
	mediaType, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil {
		return "", nil, fmt.Errorf("%w: parse content type: %w", errInvalidSessionAttachmentUpload, err)
	}
	if mediaType != "multipart/form-data" || strings.TrimSpace(params["boundary"]) == "" {
		return "", nil, fmt.Errorf(
			"%w: attachment upload requires multipart/form-data with a boundary",
			errInvalidSessionAttachmentUpload,
		)
	}
	bodyLimit := int64(math.MaxInt64)
	if maxFileBytes <= math.MaxInt64-SessionAttachmentMultipartOverheadBytes {
		bodyLimit = maxFileBytes + SessionAttachmentMultipartOverheadBytes
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit)
	reader := multipart.NewReader(c.Request.Body, params["boundary"])

	part, err := reader.NextPart()
	if err != nil {
		return "", nil, fmt.Errorf("%w: read attachment multipart field: %w", errInvalidSessionAttachmentUpload, err)
	}
	name := part.FileName()
	if part.FormName() != "file" || strings.TrimSpace(name) == "" {
		closeErr := part.Close()
		return "", nil, errors.Join(
			fmt.Errorf("%w: exactly one attachment file field is required", errInvalidSessionAttachmentUpload),
			closeErr,
		)
	}
	readLimit := maxFileBytes
	if maxFileBytes < math.MaxInt64 {
		readLimit++
	}
	data, readErr := io.ReadAll(io.LimitReader(part, readLimit))
	closeErr := part.Close()
	if readErr != nil || closeErr != nil {
		if readErr != nil && closeErr != nil {
			return "", nil, errors.Join(
				fmt.Errorf("read multipart attachment: %w", readErr),
				fmt.Errorf("close multipart attachment: %w", closeErr),
			)
		}
		if readErr != nil {
			return "", nil, fmt.Errorf("read multipart attachment: %w", readErr)
		}
		return "", nil, fmt.Errorf("close multipart attachment: %w", closeErr)
	}
	if err := attachmentspkg.ValidateSize(int64(len(data)), maxFileBytes); err != nil {
		return "", nil, err
	}
	if next, err := reader.NextPart(); !errors.Is(err, io.EOF) {
		if err == nil {
			closeErr := next.Close()
			return "", nil, errors.Join(
				fmt.Errorf("%w: exactly one attachment file field is required", errInvalidSessionAttachmentUpload),
				closeErr,
			)
		}
		return "", nil, fmt.Errorf("%w: finish multipart request: %w", errInvalidSessionAttachmentUpload, err)
	}
	return name, data, nil
}

func sessionAttachmentTooLargeError(maxFileBytes int64) error {
	return fmt.Errorf("%w: uploaded file exceeds configured max_file_bytes %d", attachmentspkg.ErrTooLarge, maxFileBytes)
}

func sessionAttachmentUploadError(err error, maxFileBytes int64) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return sessionAttachmentTooLargeError(maxFileBytes)
	}
	if errors.Is(err, attachmentspkg.ErrTooLarge) {
		return fmt.Errorf("%w: configured max_file_bytes is %d", err, maxFileBytes)
	}
	return err
}

func sessionAttachmentID(c *gin.Context) (string, error) {
	rawID := strings.TrimSpace(c.Param("attachment_id"))
	if rawID == "" {
		return "", fmt.Errorf("%w: attachment_id path is required", attachmentspkg.ErrInvalidID)
	}
	id, err := attachmentspkg.ParseAttachmentURI(attachmentspkg.AttachmentURIPrefix + rawID)
	if err != nil {
		return "", fmt.Errorf("%w: %q", attachmentspkg.ErrInvalidID, rawID)
	}
	return id, nil
}

func statusForSessionAttachmentError(err error) int {
	switch {
	case errors.Is(err, attachmentspkg.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, attachmentspkg.ErrInvalidID):
		return http.StatusBadRequest
	case errors.Is(err, errInvalidSessionAttachmentUpload):
		return http.StatusBadRequest
	case errors.Is(err, attachmentspkg.ErrTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, attachmentspkg.ErrUnsupportedMIME):
		return http.StatusUnsupportedMediaType
	default:
		return StatusForSessionError(err)
	}
}
