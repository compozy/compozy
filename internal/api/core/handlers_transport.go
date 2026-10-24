package core

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/gin-gonic/gin"
)

// HTTPPortValue returns the configured HTTP port in a concurrency-safe way.
func (h *BaseHandlers) HTTPPortValue() int {
	if h == nil {
		return 0
	}
	return int(h.httpPort.Load())
}

// StreamDoneChannel returns the transport shutdown channel in a concurrency-safe way.
func (h *BaseHandlers) StreamDoneChannel() <-chan struct{} {
	if h == nil {
		return nil
	}
	h.settingsMu.RLock()
	defer h.settingsMu.RUnlock()
	return h.streamDone
}

func (h *BaseHandlers) respondError(c *gin.Context, status int, err error) {
	h.logServerError(c, status, err)
	RespondError(c, status, err, h.MaskInternalErrors)
}

// logServerError records the cause behind a 5xx response. The access log keeps
// only the status, and the response may mask the message.
func (h *BaseHandlers) logServerError(c *gin.Context, status int, err error) {
	if err == nil {
		return
	}
	status = normalizeErrorStatus(status, err, false).status
	if status < http.StatusInternalServerError {
		return
	}
	logger := slog.Default()
	if h != nil && h.Logger != nil {
		logger = h.Logger
	}
	ctx := context.Background()
	method, path := "", ""
	if c != nil && c.Request != nil {
		ctx, method, path = c.Request.Context(), c.Request.Method, c.FullPath()
	}
	logger.ErrorContext(
		ctx, "api: request failed",
		"method", method, "path", path, "status", status,
		handlersErrorKey, diagnostics.RedactAndBound(err.Error(), 2048),
	)
}

func (h *BaseHandlers) logSessionReadError(operation string, sessionID string, err error, attrs ...any) {
	if err == nil {
		return
	}
	logger := slog.Default()
	if h != nil && h.Logger != nil {
		logger = h.Logger
	}
	logAttrs := []any{
		handlersOperationKey,
		operation,
		"session_id",
		strings.TrimSpace(sessionID),
	}
	logAttrs = append(logAttrs, attrs...)
	logAttrs = append(
		logAttrs,
		handlersErrorKey,
		err,
	)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		logger.Debug("api: session read canceled", logAttrs...)
		return
	}
	logger.Warn("api: session read failed", logAttrs...)
}

func (h *BaseHandlers) transportName() string {
	if strings.TrimSpace(h.TransportName) == "" {
		return "apicore"
	}
	return h.TransportName
}
