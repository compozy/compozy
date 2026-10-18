package httpapi

import "github.com/gin-gonic/gin"

func registerAppSDKRoutes(api gin.IRouter, handlers *Handlers) {
	api.GET("/openapi.json", handlers.GetOpenAPIDocument)
	tokens := api.Group("/app-tokens")
	privileged := handlers.privilegedMutationGuard()
	tokens.POST("", privileged, handlers.CreateAppToken)
	tokens.GET("", handlers.ListAppTokens)
	tokens.PATCH("/:app_token_id", privileged, handlers.UpdateAppToken)
	tokens.DELETE("/:app_token_id", privileged, handlers.RevokeAppToken)
}
