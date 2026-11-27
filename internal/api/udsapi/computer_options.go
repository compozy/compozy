package udsapi

import "github.com/compozy/compozy/internal/api/core"

func WithComputers(service core.ComputersService) Option {
	return func(server *Server) { server.computers = service }
}
