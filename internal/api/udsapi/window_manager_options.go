package udsapi

import (
	"github.com/compozy/compozy/internal/api/core"
	mcppkg "github.com/compozy/compozy/internal/mcp"
)

type udsExtendedServices struct {
	resources     core.ResourceService
	extensions    ExtensionService
	hostedMCP     *mcppkg.HostedService
	mcpHostAPI    mcppkg.HostAPIInvoker
	windowManager core.WindowManagerProvider
	gateway       core.GatewayService
}

// WithWindowManagerProvider injects the per-profile window managers.
func WithWindowManagerProvider(provider core.WindowManagerProvider) Option {
	return func(server *Server) {
		server.windowManager = provider
	}
}
