package udsapi

import "github.com/gin-gonic/gin"

func registerProfileRoutes(api gin.IRouter, handlers *Handlers) {
	profiles := api.Group("/profiles")
	profiles.GET("", handlers.ListProfiles)
	profiles.POST("", handlers.CreateProfile)
	profiles.GET("/ops", handlers.ListProfileOperations)
	profiles.POST("/ops/:op_id/retry", handlers.RetryProfileOperation)
	profiles.GET("/selection", handlers.GetProfileSelections)
	profiles.PUT("/selection", handlers.PutProfileSelection)
	profiles.GET("/:name", handlers.GetProfile)
	profiles.PATCH("/:name", handlers.UpdateProfile)
	profiles.GET("/:name/rename-plan", handlers.PrepareProfileRename)
	profiles.POST("/:name/rename", handlers.RenameProfile)
	profiles.GET("/:name/archive-plan", handlers.PrepareProfileArchive)
	profiles.POST("/:name/archive", handlers.ArchiveProfile)
	profiles.POST("/:name/unarchive", handlers.UnarchiveProfile)
	profiles.GET("/:name/delete-plan", handlers.PrepareProfileDelete)
	profiles.DELETE("/:name", handlers.DeleteProfile)
}
