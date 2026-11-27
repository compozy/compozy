package httpapi

import "github.com/gin-gonic/gin"

func registerNotificationRoutes(api gin.IRouter, handlers *Handlers) {
	notifications := api.Group("/notifications")
	notifications.GET("/attention", handlers.AttentionNotifications)
	notifications.POST("/attention/acknowledge", handlers.AcknowledgeAttentionNotifications)
}
