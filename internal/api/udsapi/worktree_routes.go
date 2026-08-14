package udsapi

import "github.com/gin-gonic/gin"

func registerWorktreeRoutes(api gin.IRouter, handlers *Handlers) {
	api.GET("/worktrees/catalog-stream", handlers.StreamWorktreeCatalog)
	worktrees := api.Group("/workspaces/:workspace_id/worktrees")
	{
		worktrees.GET("", handlers.ListWorktrees)
		worktrees.POST("", handlers.CreateWorktree)
		worktrees.POST("/adopt", handlers.AdoptWorktree)
		worktrees.GET("/:worktree_id", handlers.InspectWorktree)
		worktrees.GET("/:worktree_id/status", handlers.GetWorktreeStatus)
		worktrees.GET("/:worktree_id/exit", handlers.GetWorktreeExitPlan)
		worktrees.POST("/:worktree_id/exit/actions", handlers.RunWorktreeExitAction)
		worktrees.POST("/:worktree_id/exit/cancel", handlers.CancelWorktreeExitAction)
		worktrees.POST("/:worktree_id/cancel", handlers.CancelWorktreeCreate)
		worktrees.DELETE("/:worktree_id", handlers.RemoveWorktree)
		worktrees.POST("/:worktree_id/dismiss", handlers.DismissWorktree)
		worktrees.GET("/:worktree_id/stream", handlers.StreamWorktree)
	}
}
