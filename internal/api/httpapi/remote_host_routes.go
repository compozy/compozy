package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
)

func WithRemoteHost(service core.RemoteHostService) Option {
	return func(server *Server) { server.remote = service }
}

func registerRemoteHostRoutes(api gin.IRouter, handlers *Handlers) {
	routes := api.Group("/remote", handlers.privilegedMutationGuard())
	routes.GET("", handlers.RemoteStatus)
	routes.PATCH("", handlers.PatchRemoteStatus)
	routes.POST("/link", handlers.LinkRemote)
	routes.DELETE("/link", handlers.UnlinkRemote)
	routes.GET("/devices", handlers.ListRemoteDevices)
	routes.PATCH("/devices/:id", handlers.PatchRemoteDevice)
	routes.DELETE("/devices/:id", handlers.RevokeRemoteDevice)
}

type RemoteRoutes struct{ router *gin.Engine }

func NewRemoteRoutes(register func(*RemoteRoutes)) http.Handler {
	router := gin.New()
	router.UseRawPath = true // A target workspace path can contain escaped slashes in one route segment.
	router.NoRoute(func(c *gin.Context) { core.RespondRemoteError(c, remotehost.ErrRouteNotRemote) })
	register(&RemoteRoutes{router})
	return router
}

func (r *RemoteRoutes) Group(path string, scope remoteproto.Scope, middleware ...gin.HandlerFunc) *gin.RouterGroup {
	if !scope.Valid() {
		panic("invariant: remote route group requires a declared scope")
	}
	return r.router.Group(path, append([]gin.HandlerFunc{RequireScope(scope)}, middleware...)...)
}

func (r *RemoteRoutes) ToolInvoke(resolve func(string) (remoteproto.Scope, bool), handler gin.HandlerFunc) {
	r.router.POST("/api/tools/:tool_id/invoke", RequireToolScope(resolve), handler)
}

func RequireScope(scope remoteproto.Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, _ := remotehost.PrincipalFromContext(c.Request.Context())
		if err := remotehost.CheckScope(p, scope); err != nil {
			core.RespondRemoteError(c, err)
			return
		}
		c.Next()
	}
}

func RequireToolScope(resolve func(string) (remoteproto.Scope, bool)) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, _ := remotehost.PrincipalFromContext(c.Request.Context())
		if err := remotehost.CheckToolScope(p, c.Param("tool_id"), resolve); err != nil {
			core.RespondRemoteError(c, err)
			return
		}
		c.Next()
	}
}
