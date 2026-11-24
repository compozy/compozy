package core

import "github.com/gin-gonic/gin"

func RegisterComputerRoutes(api gin.IRouter, handlers *BaseHandlers, middleware ...gin.HandlerFunc) {
	routes := api.Group("/computers", middleware...)
	routes.GET("", handlers.ListComputers)
	routes.GET("/available", handlers.AvailableComputers)
	routes.GET("/stream", handlers.StreamComputers)
	routes.POST("", handlers.AddComputer)
	routes.GET("/:id", handlers.GetComputer)
	routes.PATCH("/:id", handlers.PatchComputer)
	routes.DELETE("/:id", handlers.RemoveComputer)
	routes.DELETE("/:id/routes/:route_id", handlers.RemoveComputerRoute)
}

const ComputerBridgeRoute = "/c/:computer_id/*path"
