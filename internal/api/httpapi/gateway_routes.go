package httpapi

import "github.com/gin-gonic/gin"

func registerGatewayRedeemRoute(api gin.IRouter, handlers *Handlers) {
	if !gatewayHandlersReady(handlers) {
		return
	}
	api.POST("/gateway/pairings/redeem", handlers.redeemGatewayPairing)
}

func registerGatewayManagementRoutes(api gin.IRouter, handlers *Handlers, includeStreamTickets bool) {
	if !gatewayHandlersReady(handlers) {
		return
	}
	gatewayRoutes := api.Group("/gateway")
	registerGatewayRemoteManagementRoutes(gatewayRoutes, handlers)
	gatewayRoutes.POST("/pairings", handlers.MintGatewayPairing)
	gatewayRoutes.POST("/ingress-bindings", handlers.BindGatewayIngress)
	gatewayRoutes.DELETE("/ingress-bindings/:subject_kind/:subject_id", handlers.UnbindGatewayIngress)
	if includeStreamTickets {
		gatewayRoutes.POST("/stream-tickets", handlers.MintGatewayStreamTicket)
	}
}

func registerPublicGatewayManagementRoutes(api gin.IRouter, handlers *Handlers) {
	if !gatewayHandlersReady(handlers) {
		return
	}
	gatewayRoutes := api.Group("/gateway")
	registerGatewayRemoteManagementRoutes(gatewayRoutes, handlers)
	interopGatewayStreamTicketRoute(gatewayRoutes, handlers)
}

func registerGatewayRemoteManagementRoutes(gatewayRoutes gin.IRouter, handlers *Handlers) {
	gatewayRoutes.GET("/status", handlers.GetGatewayStatus)
	gatewayRoutes.GET("/audit", handlers.GetGatewayAudit)
	gatewayRoutes.POST("/surfaces", handlers.SetGatewaySurface)
	gatewayRoutes.POST("/providers/:name/enable", handlers.EnableGatewayProvider)
	gatewayRoutes.POST("/providers/:name/disable", handlers.DisableGatewayProvider)
	gatewayRoutes.GET("/devices", handlers.ListGatewayDevices)
	gatewayRoutes.PATCH("/devices/:id", handlers.RenameGatewayDevice)
	gatewayRoutes.DELETE("/devices/:id", handlers.RevokeGatewayDevice)
}

func interopGatewayStreamTicketRoute(gatewayRoutes gin.IRouter, handlers *Handlers) {
	gatewayRoutes.POST("/stream-tickets", handlers.MintGatewayStreamTicket)
}

func gatewayHandlersReady(handlers *Handlers) bool {
	return handlers != nil && handlers.BaseHandlers != nil && handlers.Gateway != nil
}
