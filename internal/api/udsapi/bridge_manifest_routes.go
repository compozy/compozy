package udsapi

import "github.com/gin-gonic/gin"

func registerBridgeManifestRoutes(bridges gin.IRouter, handlers *Handlers) {
	bridges.GET("/providers/slack/manifest", handlers.GetSlackBridgeManifest)
}
