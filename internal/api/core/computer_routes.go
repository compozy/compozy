package core

import "github.com/gin-gonic/gin"

func RegisterComputerRoutes(api gin.IRouter, handlers *BaseHandlers, middleware ...gin.HandlerFunc) {
	routes := api.Group("/computers", middleware...)
	routes.GET("", handlers.ListComputers)
	routes.GET("/available", handlers.AvailableComputers)
	routes.GET("/stream", handlers.StreamComputers)
	routes.GET("/ssh-hosts", handlers.ComputerSSHHosts)
	routes.GET("/setups/:setup_id", handlers.ComputerSetupStatus)
	routes.DELETE("/setups/:setup_id", handlers.CancelComputerSetup)
	routes.POST("/setups/:setup_id/answer", handlers.AnswerComputerSetup)
	routes.POST("", handlers.AddComputer)
	routes.GET("/:id", handlers.GetComputer)
	routes.PATCH("/:id", handlers.PatchComputer)
	routes.POST("/:id/update", handlers.StartComputerUpdate)
	routes.DELETE("/:id", handlers.RemoveComputer)
	routes.DELETE("/:id/routes/:route_id", handlers.RemoveComputerRoute)
}

const ComputerBridgeRoute = "/c/:computer_id/*path"
