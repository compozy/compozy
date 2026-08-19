package udsapi

import "github.com/gin-gonic/gin"

func registerCmdPaletteRoutes(api gin.IRouter, handlers *Handlers) {
	palette := api.Group("/cmd-palette")
	palette.GET("/commands", handlers.ListCmdPaletteCommands)
	palette.GET("/clients", handlers.ListCmdPaletteClients)
	palette.GET("/rank-signals", handlers.GetCmdPaletteRankSignals)
	palette.GET("/personalization", handlers.GetCmdPalettePersonalization)
	palette.GET("/stream", handlers.StreamCmdPalette)
	palette.POST("/usage", handlers.RecordCmdPaletteUsage)
	palette.PUT("/pins/:id", handlers.PinCmdPaletteCommand)
	palette.DELETE("/pins/:id", handlers.UnpinCmdPaletteCommand)
	palette.DELETE("/personalization", handlers.ResetCmdPalettePersonalization)
	palette.POST("/commands/:id/invoke", handlers.InvokeCmdPaletteCommand)

	approvals := api.Group("/tools/approvals")
	approvals.GET("/:id", handlers.GetPendingToolApproval)
	approvals.POST("/:id/cancel", handlers.CancelPendingToolApproval)
}
