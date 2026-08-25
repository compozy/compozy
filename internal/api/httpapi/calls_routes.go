package httpapi

import "github.com/gin-gonic/gin"

func registerCallRoutes(api gin.IRouter, handlers *Handlers) {
	registerScopedCallRoutes(api.Group(""), handlers)
	registerScopedCallRoutes(api.Group("/workspaces/:workspace_id"), handlers)
}

func registerScopedCallRoutes(scope gin.IRouter, handlers *Handlers) {
	calls := scope.Group("/calls")
	calls.POST("", handlers.CallsCreate)
	calls.GET("", handlers.CallsList)
	calls.GET("/:call_id", handlers.CallsGet)
	calls.GET("/:call_id/prompt", handlers.CallsPrompt)
	calls.GET("/:call_id/result", handlers.CallsResult)
	calls.GET("/:call_id/superseded", handlers.CallsSuperseded)
	calls.POST("/:call_id/cancel", handlers.CallsCancel)
	calls.POST("/:call_id/await", handlers.CallsAwait)
	calls.POST("/:call_id/publish", handlers.CallsPublish)

	messages := scope.Group("/messages")
	messages.POST("", handlers.CallMessagesCreate)
	messages.GET("", handlers.CallMessagesList)
	messages.GET("/:message_id", handlers.CallMessagesGet)
}
