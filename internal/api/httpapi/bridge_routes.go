package httpapi

import "github.com/gin-gonic/gin"

func registerBridgeRoutes(api gin.IRouter, handlers *Handlers) {
	privileged := handlers.privilegedMutationGuard()
	bridges := api.Group("/bridges")
	bridges.GET("", handlers.ListBridges)
	bridges.POST("", privileged, handlers.CreateBridge)
	bridges.GET("/providers", handlers.ListBridgeProviders)
	registerBridgeManifestRoutes(api, handlers)
	registerBridgeControlRoutes(api, handlers)
	bridges.GET("/health/stream", handlers.StreamBridgeHealth)
	bridges.GET("/:id", handlers.GetBridge)
	bridges.PATCH("/:id", privileged, handlers.UpdateBridge)
	bridges.POST("/:id/enable", privileged, handlers.EnableBridge)
	bridges.POST("/:id/disable", privileged, handlers.DisableBridge)
	bridges.POST("/:id/restart", privileged, handlers.RestartBridge)
	bridges.GET("/:id/routes", handlers.ListBridgeRoutes)
	bridges.GET("/:id/targets", handlers.ListBridgeTargets)
	bridges.POST("/:id/resolve", handlers.ResolveBridgeTarget)
	bridges.GET("/:id/secret-bindings", handlers.ListBridgeSecretBindings)
	bridges.PUT("/:id/secret-bindings/:binding_name", privileged, handlers.PutBridgeSecretBinding)
	bridges.DELETE("/:id/secret-bindings/:binding_name", privileged, handlers.DeleteBridgeSecretBinding)
	bridges.POST("/:id/test-delivery", privileged, handlers.TestBridgeDelivery)
}
