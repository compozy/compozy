package udsapi

import (
	"github.com/compozy/compozy/internal/api/core"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/windowmanager"
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
