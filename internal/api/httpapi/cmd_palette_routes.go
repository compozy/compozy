package httpapi

import "github.com/compozy/compozy/internal/remoteproto"

func registerCmdPaletteRoutes(api operatorRouteGroups, handlers *Handlers) {
	palette := api.group("/cmd-palette", remoteproto.ScopeSettings)
	palette.GET("/commands", handlers.ListCmdPaletteCommands)
	palette.GET("/clients", handlers.ListCmdPaletteClients)
	palette.GET("/rank-signals", handlers.GetCmdPaletteRankSignals)
	palette.GET("/personalization", handlers.GetCmdPalettePersonalization)
	palette.GET("/stream", handlers.StreamCmdPalette)
	palette.GET("/views/:id", handlers.GetCmdPaletteView)
	palette.GET("/views/:id/stream", handlers.StreamCmdPaletteView)
	palette.POST("/views/:id/open", handlers.OpenCmdPaletteViewSession)
	palette.GET("/view-sessions/:session/stream", handlers.StreamCmdPaletteViewSession)
	palette.POST("/view-sessions/:session/events", handlers.AdmitCmdPaletteViewSessionEvent)
	palette.DELETE("/view-sessions/:session", handlers.CloseCmdPaletteViewSession)
	palette.POST("/usage", handlers.privilegedMutationGuard(), handlers.RecordCmdPaletteUsage)
	palette.PUT("/pins/:id", handlers.privilegedMutationGuard(), handlers.PinCmdPaletteCommand)
	palette.DELETE("/pins/:id", handlers.privilegedMutationGuard(), handlers.UnpinCmdPaletteCommand)
	palette.DELETE(
		"/personalization",
		handlers.privilegedMutationGuard(),
		handlers.ResetCmdPalettePersonalization,
	)
	palette.POST(
		"/commands/:id/invoke",
		handlers.privilegedMutationGuard(),
		handlers.InvokeCmdPaletteCommand,
	)

	approvals := api.group("/tools/approvals", remoteproto.ScopeSessions)
	approvals.GET("/:id", handlers.GetPendingToolApproval)
	approvals.POST("/:id/resolve", handlers.privilegedMutationGuard(), handlers.ResolvePendingToolApproval)
	approvals.POST(
		"/:id/cancel",
		handlers.privilegedMutationGuard(),
		handlers.CancelPendingToolApproval,
	)
}
