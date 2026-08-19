package udsapi

import (
	"github.com/compozy/compozy/internal/cmdpalette"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func WithCmdPalette(registry cmdpalette.Registry) Option {
	return func(server *Server) { server.cmdPalette = registry }
}

func WithApprovalCoordinator(coordinator toolspkg.ApprovalCoordinator) Option {
	return func(server *Server) { server.approvalCoordinator = coordinator }
}
