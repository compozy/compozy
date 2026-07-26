package udsapi

import "github.com/gin-gonic/gin"

func registerRoleRoutes(api gin.IRouter, handlers *Handlers) {
	roles := api.Group("/roles")
	roles.GET("", handlers.ListRoles)
	roles.GET("/:role", handlers.GetRole)
}
