package udsapi

import (
	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

func WithRemoteHost(service core.RemoteHostService) Option {
	return func(server *Server) { server.remote = service }
}

func registerRemoteHostRoutes(api gin.IRouter, handlers *Handlers) {
	routes := api.Group("/remote")
	routes.GET("", handlers.RemoteStatus)
	routes.PATCH("", handlers.PatchRemoteStatus)
	routes.POST("/link", handlers.LinkRemote)
	routes.DELETE("/link", handlers.UnlinkRemote)
	routes.GET("/devices", handlers.ListRemoteDevices)
	routes.PATCH("/devices/:id", handlers.PatchRemoteDevice)
	routes.DELETE("/devices/:id", handlers.RevokeRemoteDevice)
}
