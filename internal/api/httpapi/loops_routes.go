package httpapi

import "github.com/gin-gonic/gin"

func registerAutomationRoutes(api gin.IRouter, handlers *Handlers) {
	suggestions := api.Group("/workspaces/:workspace_id/automation/suggestions")
	suggestions.GET("", handlers.ListAutomationSuggestions)
	suggestions.POST("/:suggestion_id/accept", handlers.AcceptAutomationSuggestion)
	suggestions.POST("/:suggestion_id/dismiss", handlers.DismissAutomationSuggestion)

	automationGroup := api.Group("/automation")

	jobs := automationGroup.Group("/jobs")
	jobs.GET("", handlers.ListAutomationJobs)
	jobs.POST("", handlers.CreateAutomationJob)
	jobs.GET("/:id", handlers.GetAutomationJob)
	jobs.PATCH("/:id", handlers.UpdateAutomationJob)
	jobs.DELETE("/:id", handlers.DeleteAutomationJob)
	jobs.POST("/:id/trigger", handlers.TriggerAutomationJob)
	jobs.GET("/:id/runs", handlers.AutomationJobRuns)

	triggers := automationGroup.Group("/triggers")
	triggers.GET("", handlers.ListAutomationTriggers)
	triggers.POST("", handlers.CreateAutomationTrigger)
	triggers.GET("/:id", handlers.GetAutomationTrigger)
	triggers.PATCH("/:id", handlers.UpdateAutomationTrigger)
	triggers.DELETE("/:id", handlers.DeleteAutomationTrigger)
	triggers.GET("/:id/runs", handlers.AutomationTriggerRuns)

	runs := automationGroup.Group("/runs")
	runs.GET("", handlers.ListAutomationRuns)
	runs.GET("/:id", handlers.GetAutomationRun)
}

func registerLoopRoutes(api gin.IRouter, handlers *Handlers) {
	loops := api.Group("/workspaces/:workspace_id/loops")
	loops.GET("", handlers.ListLoops)
	loops.POST("", handlers.CreateLoop)
	loops.GET("/:name", handlers.GetLoop)
	loops.PATCH("/:name", handlers.PatchLoop)
	loops.DELETE("/:name", handlers.DeleteLoop)
	loops.POST("/:name/validate", handlers.ValidateLoop)
	loops.POST("/:name/run", handlers.RunLoop)
	loops.GET("/:name/config", handlers.GetLoopConfig)
	loops.PUT("/:name/config", handlers.PutLoopConfig)
	loops.GET("/:name/annotations", handlers.GetLoopAnnotations)
	loops.PUT("/:name/annotations", handlers.PutLoopAnnotations)

	runs := api.Group("/workspaces/:workspace_id/loop-runs")
	runs.GET("", handlers.ListLoopRuns)
	runs.GET("/:run_id", handlers.GetLoopRun)
	runs.GET("/:run_id/turns", handlers.ListGoalTurns)
	runs.POST("/:run_id/stop", handlers.StopLoopRun)
	runs.POST("/:run_id/pause", handlers.PauseLoopRun)
	runs.POST("/:run_id/resume", handlers.ResumeLoopRun)
	runs.POST("/:run_id/approve", handlers.ApproveLoopRun)
	runs.GET("/:run_id/events", handlers.StreamLoopRunEvents)
}
