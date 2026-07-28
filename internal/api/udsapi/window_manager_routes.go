package udsapi

import "github.com/gin-gonic/gin"

func registerWindowManagerRoutes(api gin.IRouter, handlers *Handlers) {
	manager := api.Group("/workspaces/:workspace_id/window-manager")
	manager.GET("", handlers.GetWindowManagerSnapshot)
	manager.POST("/preview", handlers.PreviewWindowManagerCommand)
	manager.POST("/commands", handlers.ExecuteWindowManagerCommand)
	manager.GET("/clients", handlers.ListWindowManagerClients)
	manager.POST("/clients", handlers.RegisterWindowManagerClient)
	manager.DELETE("/clients/:client_id", handlers.UnregisterWindowManagerClient)
	manager.GET("/layout", handlers.ExportWindowManagerLayout)
	manager.POST("/layout/validate", handlers.ValidateWindowManagerLayout)
	manager.PUT("/layout", handlers.ReplaceWindowManagerLayout)
	manager.GET("/layout-profiles", handlers.ListWindowManagerLayoutProfiles)
	manager.PUT("/layout-profiles/:profile_id", handlers.PutWindowManagerLayoutProfile)
	manager.DELETE("/layout-profiles/:profile_id", handlers.DeleteWindowManagerLayoutProfile)
	manager.GET("/stream", handlers.StreamWindowManager)
}
