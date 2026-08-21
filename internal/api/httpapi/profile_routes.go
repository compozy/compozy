package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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
	routes := []struct {
		method  string
		path    string
		handler gin.HandlerFunc
	}{
		{method: http.MethodPost, path: "", handler: handlers.CreateProfile},
		{method: http.MethodPut, path: "/selection", handler: handlers.PutProfileSelection},
		{method: http.MethodPost, path: "/ops/:op_id/retry", handler: handlers.RetryProfileOperation},
		{method: http.MethodPatch, path: "/:name", handler: handlers.UpdateProfile},
		{method: http.MethodPost, path: "/:name/rename", handler: handlers.RenameProfile},
		{method: http.MethodPost, path: "/:name/archive", handler: handlers.ArchiveProfile},
		{method: http.MethodPost, path: "/:name/unarchive", handler: handlers.UnarchiveProfile},
		{method: http.MethodDelete, path: "/:name", handler: handlers.DeleteProfile},
	}
	for _, route := range routes {
		if !allowMutations {
			route.handler = write
		}
		switch route.method {
		case http.MethodPost:
			profiles.POST(route.path, route.handler)
		case http.MethodPut:
			profiles.PUT(route.path, route.handler)
		case http.MethodPatch:
			profiles.PATCH(route.path, route.handler)
		case http.MethodDelete:
			profiles.DELETE(route.path, route.handler)
		}
	}
}
