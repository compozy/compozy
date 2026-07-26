package udsapi

import (
	"github.com/compozy/agh/internal/api/core"
	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/compozy/agh/internal/windowmanager"
)

type udsExtendedServices struct {
	resources     core.ResourceService
	extensions    ExtensionService
	hostedMCP     *mcppkg.HostedService
	mcpHostAPI    mcppkg.HostAPIInvoker
	windowManager windowmanager.Service
}

// WithWindowManagerService injects the daemon-authoritative window manager.
func WithWindowManagerService(service windowmanager.Service) Option {
	return func(server *Server) {
		server.windowManager = service
	}
}
