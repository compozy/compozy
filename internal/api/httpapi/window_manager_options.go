package httpapi

import (
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/windowmanager"
)

type httpExtendedServices struct {
	resources     core.ResourceService
	extensions    ExtensionService
	windowManager windowmanager.Service
}

// WithWindowManagerService injects the daemon-authoritative window manager.
func WithWindowManagerService(service windowmanager.Service) Option {
	return func(server *Server) {
		server.windowManager = service
	}
}
