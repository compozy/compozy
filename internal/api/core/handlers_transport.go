package core

import (
	"context"

	"errors"

	"log/slog"

	"strings"

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
	RespondError(c, status, err, h.MaskInternalErrors)
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
