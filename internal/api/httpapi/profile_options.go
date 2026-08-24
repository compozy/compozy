package httpapi

import "github.com/compozy/compozy/internal/api/core"

// WithProfileService injects the daemon-owned profile lifecycle service.
func WithProfileService(service core.ProfileService) Option {
	return func(server *Server) {
		server.profiles = service
	}
}
