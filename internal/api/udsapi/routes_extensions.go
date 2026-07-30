package udsapi

import "github.com/gin-gonic/gin"

func registerExtensionRoutes(api gin.IRouter, handlers *Handlers) {
	extensions := api.Group("/extensions")
	{
		extensions.GET("", handlers.ListExtensions)
		extensions.GET("/search", handlers.SearchExtensions)
		extensions.GET("/commands", handlers.ExtensionCommands)
		extensions.POST("", handlers.InstallExtension)
		extensions.POST("/dev", handlers.DevExtension)
		extensions.POST("/update", handlers.UpdateExtensions)
		extensions.PUT("/:name", handlers.UpdateExtension)
		extensions.DELETE("/:name", handlers.RemoveExtension)
		extensions.GET("/:name/provenance", handlers.ExtensionProvenance)
		extensions.GET("/:name", handlers.ExtensionStatus)
		extensions.POST("/:name/reload", handlers.ReloadDevExtension)
		extensions.GET("/:name/logs", handlers.ExtensionLogs)
		extensions.POST("/:name/enable", handlers.EnableExtension)
		extensions.POST("/:name/disable", handlers.DisableExtension)
	}
}
