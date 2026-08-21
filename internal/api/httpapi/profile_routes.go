package httpapi

import "github.com/gin-gonic/gin"

func registerProfileRoutes(api gin.IRouter, handlers *Handlers, allowMutations bool) {
	profiles := api.Group("/profiles")
	profiles.GET("", handlers.ListProfiles)
	profiles.GET("/ops", handlers.ListProfileOperations)
	profiles.GET("/selection", handlers.GetProfileSelections)
	profiles.GET("/:name", handlers.GetProfile)
	profiles.GET("/:name/rename-plan", handlers.PrepareProfileRename)
	profiles.GET("/:name/archive-plan", handlers.PrepareProfileArchive)
	profiles.GET("/:name/delete-plan", handlers.PrepareProfileDelete)

	write := handlers.ProfileRemoteWriteForbidden
	if allowMutations {
		profiles.POST("", handlers.CreateProfile)
		profiles.PUT("/selection", handlers.PutProfileSelection)
		profiles.POST("/ops/:op_id/retry", handlers.RetryProfileOperation)
		profiles.PATCH("/:name", handlers.UpdateProfile)
		profiles.POST("/:name/rename", handlers.RenameProfile)
		profiles.POST("/:name/archive", handlers.ArchiveProfile)
		profiles.POST("/:name/unarchive", handlers.UnarchiveProfile)
		profiles.DELETE("/:name", handlers.DeleteProfile)
		return
	}
	profiles.POST("", write)
	profiles.PUT("/selection", write)
	profiles.POST("/ops/:op_id/retry", write)
	profiles.PATCH("/:name", write)
	profiles.POST("/:name/rename", write)
	profiles.POST("/:name/archive", write)
	profiles.POST("/:name/unarchive", write)
	profiles.DELETE("/:name", write)
}
