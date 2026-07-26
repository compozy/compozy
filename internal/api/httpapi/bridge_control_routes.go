package httpapi

import "github.com/gin-gonic/gin"

func registerBridgeControlRoutes(api gin.IRouter, handlers *Handlers) {
	privileged := handlers.privilegedMutationGuard()
	bridges := api.Group("/bridges")
	bridges.POST("/:id/verify", privileged, handlers.VerifyBridge)
	bridges.POST("/:id/send-test", privileged, handlers.SendBridgeTest)
	bridges.POST("/:id/webhook/register", privileged, handlers.RegisterBridgeWebhook)
}
