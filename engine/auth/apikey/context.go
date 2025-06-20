package apikey

import (
	"context"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// contextKeyRequestInfo is the key for request info in context
	contextKeyRequestInfo contextKey = "apikey_request_info"
)

// RequestInfo contains HTTP request metadata for audit logging
type RequestInfo struct {
	IPAddress string
	UserAgent string
}

// WithRequestInfo adds request info to the context
func WithRequestInfo(ctx context.Context, info *RequestInfo) context.Context {
	return context.WithValue(ctx, contextKeyRequestInfo, info)
}

// GetRequestInfo retrieves request info from context
func GetRequestInfo(ctx context.Context) *RequestInfo {
	info, ok := ctx.Value(contextKeyRequestInfo).(*RequestInfo)
	if !ok {
		return &RequestInfo{} // Return empty info if not found
	}
	return info
}
