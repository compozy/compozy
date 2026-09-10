package httpapi

import "github.com/gin-gonic/gin"

func registerNotificationRoutes(api gin.IRouter, handlers *Handlers, localOnly bool) {
	privileged := handlers.privilegedMutationGuard()
	profileWrite := privileged
	if !localOnly {
		profileWrite = handlers.ProfileRemoteWriteForbidden
	}
	notifications := api.Group("/notifications")
	notifications.GET("/attention", handlers.AttentionNotifications)
	notifications.POST("/attention/acknowledge", handlers.AcknowledgeAttentionNotifications)
	presets := notifications.Group("/presets")
	presets.GET("", handlers.ListNotificationPresets)
	presets.POST("", handlers.CreateNotificationPreset)
	presets.GET("/:name", handlers.GetNotificationPreset)
	presets.PUT("/:name", handlers.UpdateNotificationPreset)
	presets.DELETE("/:name", handlers.DeleteNotificationPreset)
	presets.PUT("/:name/enablement", profileWrite, handlers.SetNotificationPresetEnablement)
}
