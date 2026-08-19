package httpapi

import "github.com/gin-gonic/gin"

func registerCmdPaletteRoutes(api gin.IRouter, handlers *Handlers) {
	palette := api.Group("/cmd-palette")
	palette.GET("/commands", handlers.ListCmdPaletteCommands)
	palette.GET("/clients", handlers.ListCmdPaletteClients)
	palette.GET("/stream", handlers.StreamCmdPalette)
	palette.POST(
		"/commands/:id/invoke",
		handlers.privilegedMutationGuard(),
		handlers.InvokeCmdPaletteCommand,
	)

	approvals := api.Group("/tools/approvals")
	approvals.GET("/:id", handlers.GetPendingToolApproval)
	approvals.POST(
		"/:id/cancel",
		handlers.privilegedMutationGuard(),
		handlers.CancelPendingToolApproval,
	)
}
