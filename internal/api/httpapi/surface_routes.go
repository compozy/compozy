package httpapi

import (
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

// RegisterSurfaceRoutes constructs one physical route matrix for the listener.
func RegisterSurfaceRoutes(router gin.IRouter, handlers *Handlers, surfaceSet SurfaceSet) {
	if handlers == nil {
		return
	}
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
	api := router.Group("/api")
	api.Use(browserRequestProtectionMiddlewareWithForwardedTarget(handlers.boundHost, true))
	api.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPrivate, gateway.SurfaceOperatorUI))
	registerGatewayRedeemRoute(api, handlers)
	api.Use(handlers.deviceAuthMiddleware())
	registerOperatorRoutes(api, handlers, false)
	registerRemoteResourceReadRoutes(api, handlers)
	registerGatewayManagementRoutes(api, handlers, true)
	registerStaticFallback(router, handlers)
}

func registerPublicIngressRoutes(router gin.IRouter, handlers *Handlers) {
	api := router.Group("/api")
	api.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPublic, gateway.SurfaceWebhookIngress))
	registerWebhookRoutes(api, handlers)
}

func registerPublicOperatorRoutes(router gin.IRouter, handlers *Handlers, includeIngress bool) {
	if includeIngress {
		ingressAPI := router.Group("/api")
		ingressAPI.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPublic, gateway.SurfaceWebhookIngress))
		registerWebhookRoutes(ingressAPI, handlers)
	}
	api := router.Group("/api")
	api.Use(browserRequestProtectionMiddlewareWithForwardedTarget(handlers.boundHost, true))
	api.Use(handlers.gatewayAdmissionMiddleware(gateway.TierPublic, gateway.SurfaceOperatorUI))
	api.Use(handlers.deviceAuthMiddleware())
	registerOperatorRoutes(api, handlers, false)
	registerRemoteResourceReadRoutes(api, handlers)
	registerPublicGatewayManagementRoutes(api, handlers)
	registerStaticFallback(router, handlers)
}

func registerOperatorRoutes(api gin.IRouter, handlers *Handlers, includeLocalOnlyTaskLifecycle bool) {
	registerStatusRoutes(api, handlers)
	registerOnboardingRoutes(api, handlers)
	registerFilesystemRoutes(api, handlers)
	registerBridgeRoutes(api, handlers)
	registerNotificationRoutes(api, handlers)
	registerProfileRoutes(api, handlers, includeLocalOnlyTaskLifecycle)
	registerWorkspaceRoutes(api, handlers)
	registerWorktreeRoutes(api, handlers)
	registerWindowManagerRoutes(api, handlers)
	registerCmdPaletteRoutes(api, handlers)
	registerSessionRoutes(api, handlers)
	registerAgentManagementRoutes(api, handlers)
	registerRoleRoutes(api, handlers)
	registerLogsRoutes(api, handlers)
	registerSupportRoutes(api, handlers)
	registerObserveRoutes(api, handlers)
	registerHookRoutes(api, handlers)
	registerToolRoutes(api, handlers)
	registerAutomationRoutes(api, handlers)
	registerLoopRoutes(api, handlers)
	registerTaskRoutes(api, handlers, includeLocalOnlyTaskLifecycle)
	registerMarketplaceRoutes(api, handlers)
	registerSkillRoutes(api, handlers)
	registerMemoryRoutes(api, handlers)
	registerNetworkRoutes(api, handlers)
	registerExtensionRoutes(api, handlers)
	registerSettingsRoutes(api, handlers)
	registerVaultRoutes(api, handlers)
	registerProviderRoutes(api, handlers)
	registerModelCatalogRoutes(api, handlers)
	registerOpenAIModelRoutes(api, handlers)
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
