package mcp

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// ResolvedServer binds one MCP config to its OAuth credential owner.
type ResolvedServer struct {
	Server compozyconfig.MCPServer
	Target mcpauth.Target
}

// ServerResolver returns the daemon-visible MCP server selected by one source.
type ServerResolver interface {
	ResolveMCPServer(ctx context.Context, source toolspkg.SourceRef) (ResolvedServer, error)
}

// ServerResolverFunc adapts a function into a server resolver.
type ServerResolverFunc func(context.Context, toolspkg.SourceRef) (ResolvedServer, error)

// ResolveMCPServer returns the configured MCP server selected by source.
func (f ServerResolverFunc) ResolveMCPServer(
	ctx context.Context,
	source toolspkg.SourceRef,
) (ResolvedServer, error) {
	if f == nil {
		return ResolvedServer{}, errors.New("mcp: server resolver is unavailable")
	}
	return f(ctx, source)
}
