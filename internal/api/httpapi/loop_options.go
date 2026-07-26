package httpapi

import "github.com/compozy/agh/internal/api/core"

// WithAutomation injects the daemon-owned automation manager.
func WithAutomation(manager core.AutomationManager) Option {
	return func(server *Server) {
		server.automation = manager
	}
}

// WithLoopService injects the daemon-owned Loop public API facade.
func WithLoopService(service core.LoopService) Option {
	return func(server *Server) {
		server.loops = service
	}
}
