package tools

import "context"

type cachedMCPProjectionKey struct{}

// WithCachedMCPProjection prevents MCP discovery while retaining live policy and cached availability.
func WithCachedMCPProjection(ctx context.Context) context.Context {
	return context.WithValue(ctx, cachedMCPProjectionKey{}, true)
}

func cachedMCPProjection(ctx context.Context) bool {
	cached, _ := ctx.Value(cachedMCPProjectionKey{}).(bool)
	return cached
}
