package core

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/store"
)

// HeaderRequestID is the canonical request ID response header.
const HeaderRequestID = "X-Request-Id"

type requestIDContextKey struct{}

// WithRequestID returns a child context carrying the transport request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
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
