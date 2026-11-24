package httpapi

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/core"
)

func WithComputers(service core.ComputersService) Option {
	return func(server *Server) { server.computers = service }
}

func (s *Server) surfaceHandler() http.Handler {
	if s.surfaceSet == SurfaceSetLocal {
		return core.AliasLocalComputer(s.engine)
	}
	return s.engine
}
