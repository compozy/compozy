package httpapi

import "github.com/gin-gonic/gin"

func registerSessionRoutes(api gin.IRouter, handlers *Handlers) {
	sessions := api.Group("/sessions")
	sessions.GET("", handlers.ListSessions)
	sessions.GET("/catalog-stream", handlers.StreamSessionCatalog)
	sessions.GET("/:session_id", handlers.GetSessionByID)
	sessions.POST("", handlers.CreateSession)

	workspaceSessions := api.Group("/workspaces/:workspace_id/sessions")
	workspaceSessions.GET("/:session_id", handlers.GetSession)
	workspaceSessions.GET("/:session_id/goal", handlers.GetSessionGoal)
	workspaceSessions.POST("/:session_id/soul/refresh", handlers.RefreshSessionSoul)
	workspaceSessions.GET("/:session_id/health", handlers.GetSessionHealth)
	workspaceSessions.GET("/:session_id/status", handlers.GetSessionStatus)
	workspaceSessions.GET("/:session_id/inspect", handlers.InspectSession)
	workspaceSessions.DELETE("/:session_id", handlers.DeleteSession)
	workspaceSessions.POST("/:session_id/stop", handlers.StopSession)
	workspaceSessions.POST("/:session_id/attach", handlers.AttachSession)
	workspaceSessions.POST("/:session_id/repair", handlers.RepairSession)
	workspaceSessions.POST("/:session_id/clear", handlers.ClearSessionConversation)
	workspaceSessions.POST("/:session_id/prompt", handlers.promptSession)
	workspaceSessions.POST("/:session_id/prompt/cancel", handlers.cancelSessionPrompt)
	workspaceSessions.POST("/:session_id/interrupt", handlers.interruptSessionPrompt)
	workspaceSessions.POST("/:session_id/steer", handlers.steerSessionPrompt)
	workspaceSessions.DELETE("/:session_id/prompt/queue/:queue_entry_id", handlers.cancelQueuedSessionPrompt)
	workspaceSessions.GET("/:session_id/events", handlers.SessionEvents)
	workspaceSessions.GET("/:session_id/history", handlers.SessionHistory)
	workspaceSessions.GET("/:session_id/transcript", handlers.SessionTranscript)
	workspaceSessions.GET("/:session_id/recap", handlers.SessionRecap)
	workspaceSessions.GET("/:session_id/usage", handlers.SessionUsage)
	workspaceSessions.GET("/:session_id/stream", handlers.StreamSession)
	workspaceSessions.POST("/:session_id/approve", handlers.approveSession)
	workspaceSessions.GET("/:session_id/clarifications", handlers.ListSessionClarifications)
	workspaceSessions.POST("/:session_id/clarifications/:request_id/answer", handlers.AnswerSessionClarification)
}
