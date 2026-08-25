package udsapi

import "github.com/compozy/compozy/internal/api/core"

// WithCallsService injects the daemon-owned calls public API facade.
func WithCallsService(service core.CallsService) Option {
	return func(server *Server) {
		server.calls = service
	}
}
