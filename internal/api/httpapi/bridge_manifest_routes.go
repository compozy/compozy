package httpapi

import "github.com/gin-gonic/gin"

func registerBridgeManifestRoutes(api gin.IRouter, handlers *Handlers) {
	bridges := api.Group("/bridges")
	bridges.GET("/providers/slack/manifest", handlers.GetSlackBridgeManifest)
}
