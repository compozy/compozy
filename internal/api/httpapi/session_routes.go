package httpapi

import "github.com/gin-gonic/gin"

func registerSessionRoutes(api gin.IRouter, handlers *Handlers) {
	registerSessionProcessRoutes(api, handlers)
	sessions := api.Group("/sessions")
	sessions.GET("", handlers.ListSessions)
	sessions.GET("/facets", handlers.ListSessionFacets)
	sessions.GET("/catalog-stream", handlers.StreamSessionCatalog)
	sessions.GET("/attention-summary", handlers.SessionAttentionSummary)
	sessions.GET("/:session_id", handlers.GetSessionByID)
	sessions.POST("/:session_id/pull-request/refresh", handlers.RefreshSessionPullRequest)
	sessions.GET("/:session_id/owner", handlers.GetSessionOwner)
	registerSessionReadRoutes(sessions, handlers)
	sessions.POST("", handlers.CreateSession)

	api.GET("/workspaces/:workspace_id/subagents/:subagent_id", handlers.GetSubagent)
	api.POST("/workspaces/:workspace_id/subagents/:subagent_id/cancel", handlers.CancelSubagent)
	registerSessionLayoutRoutes(api, handlers)
	workspaceSessions := api.Group("/workspaces/:workspace_id/sessions")
	workspaceSessions.GET("", handlers.ListSessions)
	workspaceSessions.GET("/:session_id/subagents", handlers.ListSessionSubagents)
	workspaceSessions.GET("/:session_id/monitors", handlers.ListSessionMonitors)
	workspaceSessions.POST("/:session_id/monitors/:monitor_id/cancel", handlers.CancelSessionMonitor)
	workspaceSessions.GET("/:session_id", handlers.GetSession)
	workspaceSessions.PATCH("/:session_id", handlers.RenameSession)
	registerSessionParentRoutes(workspaceSessions, handlers)
	workspaceSessions.GET("/:session_id/goal", handlers.GetSessionGoal)
	workspaceSessions.POST("/:session_id/goal", handlers.MutateSessionGoal)
	workspaceSessions.GET("/:session_id/commands", handlers.GetSessionCommands)
	registerSessionFileRoutes(workspaceSessions, handlers)
	workspaceSessions.POST("/:session_id/soul/refresh", handlers.RefreshSessionSoul)
	workspaceSessions.GET("/:session_id/health", handlers.GetSessionHealth)
	workspaceSessions.POST("/:session_id/wait", handlers.SessionWait)
	workspaceSessions.POST("/:session_id/presence", handlers.SessionPresence)
	workspaceSessions.GET("/:session_id/interactions", handlers.ListSessionInteractions)
	workspaceSessions.GET("/:session_id/inspect", handlers.InspectSession)
	workspaceSessions.DELETE("/:session_id", handlers.DeleteSession)
	workspaceSessions.POST("/:session_id/stop", handlers.StopSession)
	workspaceSessions.POST("/:session_id/archive", handlers.ArchiveSession)
	workspaceSessions.GET("/:session_id/descendants", handlers.ListSessionDescendants)
	workspaceSessions.POST("/:session_id/later", handlers.MoveSessionToLater)
	workspaceSessions.POST("/:session_id/resume-later", handlers.ResumeSessionFromLater)
	workspaceSessions.POST("/:session_id/unarchive", handlers.UnarchiveSession)
	workspaceSessions.POST("/:session_id/move", handlers.MoveSession)
	workspaceSessions.POST("/:session_id/attach", handlers.AttachSession)
	workspaceSessions.POST("/:session_id/worktree-fork", handlers.ForkSessionToWorktree)
	workspaceSessions.PUT("/:session_id/worktree", handlers.BindSessionWorktree)
	workspaceSessions.DELETE("/:session_id/worktree", handlers.UnbindSessionWorktree)
	workspaceSessions.POST("/:session_id/repair", handlers.RepairSession)
	workspaceSessions.POST("/:session_id/compact", handlers.CompactSession)
	workspaceSessions.POST("/:session_id/clear", handlers.ClearSessionConversation)
	workspaceSessions.POST("/:session_id/rewind", handlers.RewindSessionConversation)
	workspaceSessions.POST("/:session_id/continue", handlers.ContinueSession)
	workspaceSessions.POST("/:session_id/fork", handlers.ForkSession)
	workspaceSessions.GET("/:session_id/derive/preview", handlers.PreviewSessionDerive)
	workspaceSessions.PUT("/:session_id/runtime", handlers.SetSessionRuntime)
	workspaceSessions.DELETE("/:session_id/runtime", handlers.ClearSessionRuntime)
	registerSessionPromptRoutes(workspaceSessions, handlers)
	registerSessionReadRoutes(workspaceSessions, handlers)
	workspaceSessions.GET("/:session_id/recap", handlers.SessionRecap)
	workspaceSessions.GET("/:session_id/usage", handlers.SessionUsage)
	workspaceSessions.GET("/:session_id/usage/turns", handlers.SessionUsageTurns)
	workspaceSessions.POST("/:session_id/approve", handlers.approveSession)
	workspaceSessions.GET("/:session_id/clarifications", handlers.ListSessionClarifications)
	workspaceSessions.POST("/:session_id/clarifications/:request_id/answer", handlers.AnswerSessionClarification)
}

func registerSessionProcessRoutes(api gin.IRouter, handlers *Handlers) {
	api.GET("/processes", handlers.ListProcesses)
	api.POST("/processes/terminate", handlers.TerminateProcesses)
	workspaceSessions := api.Group("/workspaces/:workspace_id/sessions")
	workspaceSessions.GET("/:session_id/processes", handlers.ListSessionProcesses)
	workspaceSessions.POST("/:session_id/processes/terminate", handlers.TerminateSessionProcesses)
}

func registerSessionReadRoutes(sessions gin.IRouter, handlers *Handlers) {
	sessions.GET("/:session_id/stream", handlers.StreamSession)
	sessions.GET("/:session_id/history", handlers.SessionHistory)
	sessions.GET("/:session_id/events", handlers.SessionEvents)
	sessions.GET("/:session_id/status", handlers.GetSessionStatus)
	sessions.GET("/:session_id/transcript", handlers.SessionTranscript)
	sessions.GET("/:session_id/transcript/search", handlers.SessionTranscriptSearch)
	sessions.GET("/:session_id/transcript/outline", handlers.SessionTranscriptOutline)
}

func registerSessionLayoutRoutes(api gin.IRouter, handlers *Handlers) {
	folders := api.Group("/workspaces/:workspace_id/session-folders")
	folders.GET("", handlers.ListSessionFolders)
	folders.POST("", handlers.CreateSessionFolder)
	folders.PATCH("/:folder_id", handlers.RenameSessionFolder)
	folders.DELETE("/:folder_id", handlers.DeleteSessionFolder)
	order := api.Group("/workspaces/:workspace_id/session-order")
	order.PUT("/folders", handlers.ReorderSessionFolders)
	order.PUT("/sessions", handlers.ReorderSessions)
	workspaceFolders := api.Group("/workspace-folders")
	workspaceFolders.GET("", handlers.GetWorkspaceLayout)
	workspaceFolders.POST("", handlers.CreateWorkspaceFolder)
	workspaceFolders.PATCH("/:folder_id", handlers.RenameWorkspaceFolder)
	workspaceFolders.DELETE("/:folder_id", handlers.DeleteWorkspaceFolder)
	workspaceOrder := api.Group("/workspace-order")
	workspaceOrder.PUT("/folders", handlers.ReorderWorkspaceFolders)
	workspaceOrder.PUT("/workspaces", handlers.ReorderWorkspaces)
	workspaceOrder.POST("/move", handlers.MoveWorkspace)
}

func registerSessionFileRoutes(workspaceSessions gin.IRouter, handlers *Handlers) {
	workspaceSessions.GET("/:session_id/files", handlers.SearchSessionFiles)
	workspaceSessions.POST("/:session_id/attachments", handlers.UploadSessionAttachment)
	workspaceSessions.GET("/:session_id/attachments/:attachment_id/bytes", handlers.ReadSessionAttachmentBytes)
	workspaceSessions.DELETE("/:session_id/attachments/:attachment_id", handlers.DeleteSessionAttachment)
}

func registerSessionParentRoutes(workspaceSessions gin.IRouter, handlers *Handlers) {
	workspaceSessions.PUT("/:session_id/parent", handlers.SetSessionParent)
	workspaceSessions.DELETE("/:session_id/parent", handlers.ClearSessionParent)
}

func registerSessionPromptRoutes(workspaceSessions gin.IRouter, handlers *Handlers) {
	workspaceSessions.POST("/:session_id/prompt", handlers.promptSession)
	workspaceSessions.POST("/:session_id/prompt/cancel", handlers.CancelSessionPrompt)
	workspaceSessions.POST("/:session_id/steer", handlers.steerSessionPrompt)
	workspaceSessions.GET("/:session_id/prompt/queue", handlers.listSessionInputs)
	workspaceSessions.DELETE("/:session_id/prompt/queue", handlers.HandleClearSessionInputs)
	workspaceSessions.PUT("/:session_id/prompt/queue/:queue_entry_id", handlers.replaceSessionInput)
	workspaceSessions.POST("/:session_id/prompt/queue/:queue_entry_id/steer", handlers.promoteSessionInput)
	workspaceSessions.DELETE("/:session_id/prompt/queue/:queue_entry_id", handlers.cancelQueuedSessionPrompt)
}
