package httpapi

import (
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/gin-gonic/gin"
)

// RegisterSurfaceRoutes constructs one physical route matrix for the listener.
func RegisterSurfaceRoutes(router gin.IRouter, handlers *Handlers, surfaceSet SurfaceSet) {
	if handlers == nil {
		return
	}
	router.Use(core.ProfileLabels)
	switch surfaceSet {
	case SurfaceSetLocal:
		registerLocalRoutes(router, handlers)
	case SurfaceSetPrivate:
		registerPrivateTierRoutes(router, handlers)
	case SurfaceSetPublicIngress:
		registerPublicIngressRoutes(router, handlers)
	case SurfaceSetPublicOperator:
		registerPublicOperatorRoutes(router, handlers, false)
	case SurfaceSetPublicCombined:
		registerPublicOperatorRoutes(router, handlers, true)
	}
}

func registerPrivateTierRoutes(router gin.IRouter, handlers *Handlers) {
	signIn := core.NewSignInRoutes(router, handlers.BaseHandlers)
	api := router.Group("/api")
	api.Use(browserRequestProtectionMiddlewareWithForwardedTarget(handlers.boundHost, true))
	api.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPrivate, gateway.SurfaceOperatorUI))
	signIn.RequireSignedIn(core.SignInLocal, func() { registerGatewayRedeemRoute(api, handlers) })
	api.Use(handlers.deviceAuthMiddleware())
	api.Use(handlers.WorkspaceReferenceGuard)
	signIn.RequireSignedIn(core.SignInAnonymous, func() {
		registerSignedOutOperatorRoutes(operatorRouteGroups{router: api}, handlers)
	})
	signIn.RequireSignedIn(core.SignInLocal, func() {
		registerOperatorRoutes(operatorRouteGroups{router: api}, handlers, false)
		registerRemoteResourceReadRoutes(api, handlers)
		registerGatewayManagementRoutes(api, handlers, true)
	})
	signIn.Validate()
	registerStaticFallback(router, handlers)
}

func registerPublicIngressRoutes(router gin.IRouter, handlers *Handlers) {
	signIn := core.NewSignInRoutes(router, handlers.BaseHandlers)
	api := router.Group("/api")
	api.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPublic, gateway.SurfaceWebhookIngress))
	signIn.RequireSignedIn(core.SignInAnonymous, func() { registerWebhookRoutes(api, handlers) })
	signIn.Validate()
}

func registerPublicOperatorRoutes(router gin.IRouter, handlers *Handlers, includeIngress bool) {
	signIn := core.NewSignInRoutes(router, handlers.BaseHandlers)
	if includeIngress {
		ingressAPI := router.Group("/api")
		ingressAPI.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPublic, gateway.SurfaceWebhookIngress))
		signIn.RequireSignedIn(core.SignInAnonymous, func() { registerWebhookRoutes(ingressAPI, handlers) })
	}
	api := router.Group("/api")
	api.Use(browserRequestProtectionMiddlewareWithForwardedTarget(handlers.boundHost, true))
	api.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPublic, gateway.SurfaceOperatorUI))
	api.Use(handlers.deviceAuthMiddleware())
	api.Use(handlers.WorkspaceReferenceGuard)
	signIn.RequireSignedIn(core.SignInAnonymous, func() {
		registerSignedOutOperatorRoutes(operatorRouteGroups{router: api}, handlers)
	})
	signIn.RequireSignedIn(core.SignInLocal, func() {
		registerOperatorRoutes(operatorRouteGroups{router: api}, handlers, false)
		registerRemoteResourceReadRoutes(api, handlers)
		registerPublicGatewayManagementRoutes(api, handlers)
	})
	signIn.Validate()
	registerStaticFallback(router, handlers)
}

func registerSignedOutOperatorRoutes(api operatorRouteGroups, handlers *Handlers) {
	status := api.group("/status", remoteproto.ScopeSessions)
	status.GET("", handlers.GetStatus)
	status.GET("/identity", handlers.GetRuntimeIdentity)

	settings := api.group("/settings", remoteproto.ScopeSettings)
	privileged := handlers.privilegedMutationGuard()
	settings.GET("/update", handlers.GetSettingsUpdate)
	settings.POST("/update/check", privileged, handlers.CheckSettingsUpdate)
	settings.POST("/update/apply", privileged, handlers.ApplySettingsUpdate)
	settings.POST("/update/cancel", privileged, handlers.CancelSettingsUpdate)
	settings.POST("/actions/restart", privileged, handlers.TriggerSettingsRestart)
	settings.GET("/actions/restart/:operation_id", handlers.GetSettingsRestartStatus)
}

func registerOperatorRoutes(api operatorRouteGroups, handlers *Handlers, fullOperator bool) {
	registerStatusRoutes(api, handlers)
	registerOnboardingRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerFilesystemRoutes(api.group("", remoteproto.ScopeFilesR), handlers)
	registerNotificationRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerProfileRoutes(api, handlers, fullOperator)
	registerWorkspaceRoutes(api, handlers)
	registerWorktreeRoutes(api, handlers)
	registerWindowManagerRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	if fullOperator {
		registerTerminalRoutes(api.group("", remoteproto.ScopeTerminal), handlers)
	}
	registerCmdPaletteRoutes(api, handlers)
	registerSessionRoutes(api, handlers)
	registerAgentManagementRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerRoleRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerLogsRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerSupportRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerObserveRoutes(api, handlers)
	registerHookRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerToolRoutes(api, handlers)
	registerAutomationRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerLoopRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerTaskRoutes(api.group("", remoteproto.ScopeTasks), handlers, fullOperator, api.remote)
	registerMarketplaceRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerSkillRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerExtensionRoutes(api.group("", remoteproto.ScopeSettings), handlers, fullOperator)
	registerSettingsRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerVaultRoutes(api, handlers)
	registerProviderRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerModelCatalogRoutes(api, handlers)
	registerProviderUsageRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerOpenAIModelRoutes(api.group("", remoteproto.ScopeSessions), handlers)
}

func registerStaticFallback(router gin.IRouter, handlers *Handlers) {
	if engine, ok := router.(*gin.Engine); ok {
		authenticateAPI := handlers.deviceAuthMiddleware()
		engine.NoRoute(
			browserRequestProtectionMiddlewareWithForwardedTarget(handlers.boundHost, true),
			func(c *gin.Context) {
				if isStaticBypassPath(normalizedRequestPath(c.Request.URL.Path)) {
					authenticateAPI(c)
					return
				}
				c.Next()
			},
			handlers.serveStaticRoute,
		)
	}
}
