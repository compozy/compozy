package core

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the shared daemon API routes on the supplied router.
func RegisterRoutes(router gin.IRouter, handlers *Handlers) {
	if router == nil || handlers == nil {
		return
	}

	api := router.Group("/api")

	daemon := api.Group("/daemon")
	daemon.GET("/status", handlers.DaemonStatus)
	daemon.GET("/health", handlers.DaemonHealth)
	daemon.GET("/metrics", handlers.DaemonMetrics)
	daemon.POST("/stop", handlers.StopDaemon)

	workspaces := api.Group("/workspaces")
	workspaces.POST("", handlers.RegisterWorkspace)
	workspaces.GET("", handlers.ListWorkspaces)
	workspaces.GET("/:id", handlers.GetWorkspace)
	workspaces.PATCH("/:id", handlers.UpdateWorkspace)
	workspaces.DELETE("/:id", handlers.DeleteWorkspace)
	workspaces.POST("/resolve", handlers.ResolveWorkspace)

	ui := api.Group("/ui")
	ui.GET("/dashboard", handlers.GetDashboard)

	tasks := api.Group("/tasks")
	tasks.GET("", handlers.ListTaskWorkflows)
	tasks.GET("/:slug", handlers.GetTaskWorkflow)
	tasks.GET("/:slug/spec", handlers.GetWorkflowSpec)
	tasks.GET("/:slug/memory", handlers.GetWorkflowMemory)
	tasks.GET("/:slug/memory/files/:file_id", handlers.GetWorkflowMemoryFile)
	tasks.GET("/:slug/board", handlers.GetTaskBoard)
	tasks.GET("/:slug/items", handlers.ListTaskItems)
	tasks.GET("/:slug/items/:task_id", handlers.GetTaskItemDetail)
	tasks.POST("/:slug/validate", handlers.ValidateTaskWorkflow)
	tasks.POST("/:slug/runs", handlers.StartTaskRun)
	tasks.POST("/:slug/archive", handlers.ArchiveTaskWorkflow)

	reviews := api.Group("/reviews")
	reviews.POST("/:slug/fetch", handlers.FetchReview)
	reviews.GET("/:slug/rounds/:round/issues", handlers.ListReviewIssues)
	reviews.GET("/:slug/rounds/:round/issues/:issue_id", handlers.GetReviewIssue)
	reviews.GET("/:slug/rounds/:round", handlers.GetReviewRound)
	reviews.POST("/:slug/rounds/:round/runs", handlers.StartReviewRun)
	reviews.GET("/:slug", handlers.GetLatestReview)

	runs := api.Group("/runs")
	runs.GET("", handlers.ListRuns)
	runs.GET("/:run_id", handlers.GetRun)
	runs.GET("/:run_id/snapshot", handlers.GetRunSnapshot)
	runs.GET("/:run_id/events", handlers.ListRunEvents)
	runs.GET("/:run_id/stream", handlers.StreamRun)
	runs.POST("/:run_id/cancel", handlers.CancelRun)

	api.POST("/sync", handlers.SyncWorkflow)
	api.POST("/exec", handlers.StartExecRun)
}
