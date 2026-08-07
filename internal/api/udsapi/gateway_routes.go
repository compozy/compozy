package udsapi

import "github.com/gin-gonic/gin"

func registerGatewayRoutes(api gin.IRouter, handlers *Handlers) {
	if handlers == nil || handlers.BaseHandlers == nil || handlers.Gateway == nil {
		return
	}
	routes := api.Group("/gateway")
	routes.GET("/status", handlers.GetGatewayStatus)
	routes.POST("/surfaces", handlers.SetGatewaySurface)
	routes.POST("/providers/:name/enable", handlers.EnableGatewayProvider)
	routes.POST("/providers/:name/disable", handlers.DisableGatewayProvider)
	routes.POST("/pairings", handlers.MintGatewayPairing)
	routes.POST("/pairings/redeem", handlers.RedeemGatewayPairing)
	routes.GET("/devices", handlers.ListGatewayDevices)
	routes.PATCH("/devices/:id", handlers.RenameGatewayDevice)
	routes.DELETE("/devices/:id", handlers.RevokeGatewayDevice)
	routes.POST("/ingress-bindings", handlers.BindGatewayIngress)
	routes.DELETE("/ingress-bindings/:subject_kind/:subject_id", handlers.UnbindGatewayIngress)
}
