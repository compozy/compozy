package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/ginutil"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/tools/builtin"
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

// operatorRouteGroups is the shared entry point for local and remote operator
// route registration. Remote groups never expose the unscoped root router.
type operatorRouteGroups struct {
	router gin.IRouter
	remote bool
}

type RemoteRoutes struct{ router *gin.Engine }

func NewRemoteRoutes(register func(*RemoteRoutes)) http.Handler {
	return buildRemoteRoutes(false, register)
}

func (r *RemoteRoutes) Group(path string, scope remoteproto.Scope, middleware ...gin.HandlerFunc) *gin.RouterGroup {
	group := (operatorRouteGroups{router: r.router, remote: true}).group(path, scope)
	group.Use(middleware...)
	return group
}

// NewRemoteOperatorHandler uses the same dependencies as the local HTTP API.
// The remote host authenticates the request and owns its lifetime before dispatch.
func NewRemoteOperatorHandler(browser bool, opts ...Option) http.Handler {
	server := &Server{}
	applyOptions(server, opts)
	server.applyDefaults()
	server.configureAddress()
	handlers := newHandlers(server.handlerConfig(nil))
	return newRemoteOperatorRouter(handlers, browser)
}

func newRemoteOperatorRouter(handlers *Handlers, browser bool) *gin.Engine {
	return buildRemoteRoutes(browser, func(routes *RemoteRoutes) {
		routes.router.Use(requestBodyLimitMiddlewareWithUploadLimit(
			maxAPIRequestBodyBytes,
			core.SessionAttachmentRequestBodyLimit(handlers.Config.Session.Attachments.MaxFileBytes),
		))
		api := routes.router.Group("/api", handlers.WorkspaceReferenceGuard)
		operator := operatorRouteGroups{router: api, remote: true}
		registerOperatorRoutes(operator, handlers, true)
		admin := operator.group("", remoteproto.ScopeAdmin)
		registerRemoteHostRoutes(admin, handlers)
		core.RegisterIngressRoutes(admin, handlers.BaseHandlers, handlers.privilegedMutationGuard())
		registerAppTokenRoutes(admin, handlers)
		operator.group("", remoteproto.ScopeSessions).GET("/openapi.json", handlers.GetOpenAPIDocument)
		registerResourceEndpoints(operator.group("", remoteproto.ScopeSettings), handlers, true)
	})
}

func buildRemoteRoutes(browser bool, register func(*RemoteRoutes)) *gin.Engine {
	router := ginutil.NewEngine()
	router.RedirectTrailingSlash = false
	router.Use(gin.Recovery(), remoteOperatorBoundary(browser), core.ProfileLabels)
	ginutil.QuietDebug(func() {
		register(&RemoteRoutes{router})
	})
	router.NoRoute(func(c *gin.Context) { core.RespondRemoteError(c, remotehost.ErrRouteNotRemote) })
	return router
}

func remoteOperatorBoundary(browser bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := remotehost.RejectClaimToken(c.Request); err != nil {
			core.RespondRemoteError(c, err)
			return
		}
		if err := remotehost.OperatorRouteRejection(c.Request.URL.Path, browser); err != nil {
			core.RespondRemoteError(c, err)
			return
		}
		// Agent caller headers are local credentials. A remote request can only
		// act as its authenticated device, including over an unbridged tunnel.
		c.Request.Header.Del(agentidentity.HeaderSessionID)
		c.Request.Header.Del(agentidentity.HeaderAgent)
		c.Request = c.Request.WithContext(toolspkg.WithoutComputerForwarding(c.Request.Context()))
		c.Next()
	}
}

func (r operatorRouteGroups) group(path string, scope remoteproto.Scope) *gin.RouterGroup {
	if !scope.Valid() {
		panic("invariant: remote route group requires a declared scope")
	}
	if r.remote {
		return r.router.Group(path, RequireScope(scope))
	}
	return r.router.Group(path)
}

func (r operatorRouteGroups) toolInvoke(privileged, handler gin.HandlerFunc) {
	const path = "/tools/:id/invoke"
	if r.remote {
		r.router.POST(path, RequireToolScope(builtin.RemoteToolScope), handler)
		return
	}
	r.router.POST(path, privileged, handler)
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
		if err := remotehost.CheckToolScope(p, c.Param("id"), resolve); err != nil {
			core.RespondRemoteError(c, err)
			return
		}
		c.Next()
	}
}
