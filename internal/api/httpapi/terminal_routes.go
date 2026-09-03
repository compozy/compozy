package httpapi

import "github.com/gin-gonic/gin"

func registerTerminalRoutes(api gin.IRouter, handlers *Handlers) {
	terminals := api.Group("/workspaces/:workspace_id/terminals")
	terminals.GET("/stream", handlers.StreamTerminalCatalog)
	terminals.GET("", handlers.ListTerminals)
	terminals.POST("", handlers.CreateTerminal)
	terminals.POST("/exec", handlers.ExecTerminal)
	terminals.GET("/input-requests", handlers.ListTerminalInputRequests)
	terminals.GET("/journal", handlers.QueryTerminalJournal)
	terminals.GET("/recordings/:id", handlers.DownloadTerminalRecording)
	terminals.GET("/artifacts/:id", handlers.DownloadTerminalArtifact)
	terminals.GET("/:id", handlers.GetTerminal)
	terminals.DELETE("/:id", handlers.DeleteTerminal)
	terminals.POST("/:id/attach-ticket", handlers.MintTerminalAttachTicket)
	terminals.GET("/:id/stream", handlers.StreamTerminal)
	terminals.GET("/:id/read", handlers.ReadTerminal)
	terminals.POST("/:id/signal", handlers.SignalTerminal)
	terminals.POST("/:id/wait", handlers.WaitTerminal)
	terminals.POST("/:id/input-requests/:request_id/answer", handlers.AnswerTerminalInputRequest)
	terminals.POST("/:id/input-requests/:request_id/reject", handlers.RejectTerminalInputRequest)
	terminals.POST("/:id/recording", handlers.ControlTerminalRecording)
}
