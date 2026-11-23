package udsapi

import "github.com/compozy/compozy/internal/api/core"

// WithGatewayService injects operator-global gateway management handlers.
func WithGatewayService(service core.GatewayService) Option {
	return func(server *Server) {
		server.gateway = service
	}
}

// WithIngressService installs the webhook ingress binding service.
func WithIngressService(service core.IngressService) Option {
	return func(server *Server) {
		server.ingress = service
	}
}
