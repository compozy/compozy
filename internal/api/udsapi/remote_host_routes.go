package udsapi

import (
	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

func WithRemoteHost(service core.RemoteHostService) Option {
	return func(server *Server) { server.remote = service }
}

func registerRemoteHostRoutes(api gin.IRouter, handlers *Handlers, signIn *core.SignInRoutes) {
	core.RegisterRemoteHostRoutes(api.Group("/remote"), handlers.BaseHandlers, signIn)
}
