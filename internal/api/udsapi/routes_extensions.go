package udsapi

import "github.com/gin-gonic/gin"

func registerExtensionRoutes(api gin.IRouter, handlers *Handlers) {
	extensions := api.Group("/extensions")
	{
		extensions.GET("", handlers.ListExtensions)
		extensions.GET("/search", handlers.SearchExtensions)
		extensions.GET("/commands", handlers.ExtensionCommands)
		extensions.POST("/preview-install", handlers.PreviewExtensionInstall)
		extensions.POST("", handlers.InstallExtension)
		extensions.POST("/dev", handlers.DevExtension)
		extensions.POST("/update", handlers.UpdateExtensions)
		extensions.PUT("/:name", handlers.UpdateExtension)
		extensions.DELETE("/:name", handlers.RemoveExtension)
		extensions.GET("/:name/provenance", handlers.ExtensionProvenance)
		extensions.GET("/:name/secrets", handlers.ListExtensionSecrets)
		extensions.PUT("/:name/secrets", handlers.SetExtensionSecrets)
		extensions.DELETE("/:name/secrets/:env_name", handlers.DeleteExtensionSecret)
		extensions.GET("/:name/inventory", handlers.ExtensionInventory)
		extensions.GET("/:name", handlers.ExtensionStatus)
		extensions.POST("/:name/reload", handlers.ReloadDevExtension)
		extensions.GET("/:name/logs", handlers.ExtensionLogs)
		extensions.GET("/:name/enablement", handlers.ListExtensionEnablement)
		extensions.PUT("/:name/enablement", handlers.SetExtensionEnablement)
		extensions.GET("/:name/preview", handlers.PreviewExtensionEnable)
	}
}
