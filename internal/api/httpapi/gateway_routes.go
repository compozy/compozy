package httpapi

import "github.com/gin-gonic/gin"

func registerGatewayRedeemRoute(api gin.IRouter, handlers *Handlers) {
	if handlers == nil || handlers.BaseHandlers == nil || handlers.Gateway == nil {
		return
	}
	api.POST("/gateway/pairings/redeem", handlers.redeemGatewayPairing)
}

func registerGatewayManagementRoutes(api gin.IRouter, handlers *Handlers, includeStreamTickets bool) {
	if handlers == nil || handlers.BaseHandlers == nil || handlers.Gateway == nil {
		return
	}
	gatewayRoutes := api.Group("/gateway")
	gatewayRoutes.GET("/status", handlers.GetGatewayStatus)
	gatewayRoutes.GET("/audit", handlers.GetGatewayAudit)
	gatewayRoutes.POST("/surfaces", handlers.SetGatewaySurface)
	gatewayRoutes.POST("/providers/:name/enable", handlers.EnableGatewayProvider)
	gatewayRoutes.POST("/providers/:name/disable", handlers.DisableGatewayProvider)
	gatewayRoutes.POST("/pairings", handlers.MintGatewayPairing)
	gatewayRoutes.GET("/devices", handlers.ListGatewayDevices)
	gatewayRoutes.PATCH("/devices/:id", handlers.RenameGatewayDevice)
	gatewayRoutes.DELETE("/devices/:id", handlers.RevokeGatewayDevice)
	gatewayRoutes.POST("/ingress-bindings", handlers.BindGatewayIngress)
	gatewayRoutes.DELETE("/ingress-bindings/:subject_kind/:subject_id", handlers.UnbindGatewayIngress)
	if includeStreamTickets {
		gatewayRoutes.POST("/stream-tickets", handlers.MintGatewayStreamTicket)
	}
}

func registerGatewayStreamTicketRoute(api gin.IRouter, handlers *Handlers) {
	if handlers == nil || handlers.BaseHandlers == nil || handlers.Gateway == nil {
		return
	}
	api.POST("/gateway/stream-tickets", handlers.MintGatewayStreamTicket)
}
