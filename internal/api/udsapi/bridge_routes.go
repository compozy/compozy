package udsapi

import "github.com/gin-gonic/gin"

func registerBridgeRoutes(api gin.IRouter, handlers *Handlers) {
	bridges := api.Group("/bridges")
	bridges.GET("", handlers.ListBridges)
	bridges.POST("", handlers.CreateBridge)
	bridges.GET("/providers", handlers.ListBridgeProviders)
	registerBridgeManifestRoutes(bridges, handlers)
	registerBridgeControlRoutes(bridges, handlers)
	bridges.GET("/health/stream", handlers.StreamBridgeHealth)
	bridges.GET("/:id", handlers.GetBridge)
	bridges.PATCH("/:id", handlers.UpdateBridge)
	bridges.POST("/:id/enable", handlers.EnableBridge)
	bridges.POST("/:id/disable", handlers.DisableBridge)
	bridges.POST("/:id/restart", handlers.RestartBridge)
	bridges.GET("/:id/routes", handlers.ListBridgeRoutes)
	bridges.GET("/:id/targets", handlers.ListBridgeTargets)
	bridges.POST("/:id/resolve", handlers.ResolveBridgeTarget)
	bridges.GET("/:id/secret-bindings", handlers.ListBridgeSecretBindings)
	bridges.PUT("/:id/secret-bindings/:binding_name", handlers.PutBridgeSecretBinding)
	bridges.DELETE("/:id/secret-bindings/:binding_name", handlers.DeleteBridgeSecretBinding)
	bridges.POST("/:id/test-delivery", handlers.TestBridgeDelivery)
}
