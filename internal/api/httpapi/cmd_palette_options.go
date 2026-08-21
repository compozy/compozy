package httpapi

import (
	"github.com/compozy/compozy/internal/cmdpalette"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// WithCmdPalette injects the command-palette registry into the HTTP server.
func WithCmdPalette(registry cmdpalette.Registry) Option {
	return func(server *Server) { server.cmdPalette = registry }
}

// WithApprovalCoordinator injects the asynchronous tool-approval coordinator.
func WithApprovalCoordinator(coordinator toolspkg.ApprovalCoordinator) Option {
	return func(server *Server) { server.approvalCoordinator = coordinator }
}
