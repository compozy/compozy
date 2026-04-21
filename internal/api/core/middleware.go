package core

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/store"
)

// HeaderRequestID is the canonical request ID response header.
const HeaderRequestID = "X-Request-Id"

// HeaderActiveWorkspaceID carries the browser-selected active workspace ID.
const HeaderActiveWorkspaceID = "X-Compozy-Workspace-ID"

// HeaderCSRF carries the double-submit CSRF header name for browser mutations.
const HeaderCSRF = "X-Compozy-CSRF-" + "Token"

// CookieCSRF carries the browser mutation CSRF cookie name.
const CookieCSRF = "compozy" + "_csrf"

type requestIDContextKey struct{}
type activeWorkspaceContextKey struct{}

// WithRequestID returns a child context carrying the transport request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, strings.TrimSpace(requestID))
}

// RequestIDFromContext returns the propagated request ID when available.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, ok := ctx.Value(requestIDContextKey{}).(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// WithActiveWorkspaceID returns a child context carrying the resolved active workspace ID.
func WithActiveWorkspaceID(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, activeWorkspaceContextKey{}, strings.TrimSpace(workspaceID))
}

// ActiveWorkspaceIDFromContext returns the resolved active workspace ID when available.
func ActiveWorkspaceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, ok := ctx.Value(activeWorkspaceContextKey{}).(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// RequestIDMiddleware ensures every request carries a transport request ID.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(HeaderRequestID))
		if requestID == "" {
			requestID = store.NewID("req")
		}

		c.Writer.Header().Set(HeaderRequestID, requestID)
		c.Request = c.Request.WithContext(WithRequestID(c.Request.Context(), requestID))
		c.Next()
	}
}

// ErrorMiddleware converts unhandled Gin errors into the transport error envelope.
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		RespondError(c, c.Errors.Last().Err)
	}
}
