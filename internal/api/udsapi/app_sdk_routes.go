package udsapi

import "github.com/gin-gonic/gin"

func registerAppSDKRoutes(api gin.IRouter, handlers *Handlers) {
	api.GET("/openapi.json", handlers.GetOpenAPIDocument)
	tokens := api.Group("/app-tokens")
	tokens.POST("", handlers.CreateAppToken)
	tokens.GET("", handlers.ListAppTokens)
	tokens.PATCH("/:app_token_id", handlers.UpdateAppToken)
	tokens.DELETE("/:app_token_id", handlers.RevokeAppToken)
}
