package udsapi

import "github.com/gin-gonic/gin"

func registerBridgeControlRoutes(bridges gin.IRouter, handlers *Handlers) {
	bridges.POST("/:id/verify", handlers.VerifyBridge)
	bridges.POST("/:id/send-test", handlers.SendBridgeTest)
	bridges.POST("/:id/webhook/register", handlers.RegisterBridgeWebhook)
}
