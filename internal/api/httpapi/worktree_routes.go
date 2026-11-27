package httpapi

import "github.com/compozy/compozy/internal/remoteproto"

func registerWorktreeRoutes(api operatorRouteGroups, handlers *Handlers) {
	aggregate := api.group("/worktrees", remoteproto.ScopeFilesR)
	aggregate.GET("", handlers.ListWorktreeAggregate)
	aggregate.GET("/catalog-stream", handlers.StreamWorktreeCatalog)
	const path = "/workspaces/:workspace_id/worktrees"
	worktrees := api.group(path, remoteproto.ScopeFilesR)
	worktrees.GET("", handlers.ListWorktrees)
	worktrees.GET("/:worktree_id", handlers.InspectWorktree)
	worktrees.GET("/:worktree_id/status", handlers.GetWorktreeStatus)
	worktrees.GET("/:worktree_id/exit", handlers.GetWorktreeExitPlan)
	worktrees.GET("/:worktree_id/stream", handlers.StreamWorktree)

	writes := api.group(path, remoteproto.ScopeFilesW)
	writes.POST("", handlers.CreateWorktree)
	writes.POST("/adopt", handlers.AdoptWorktree)
	writes.POST("/discovered/remove", handlers.RemoveDiscoveredWorktree)
	writes.POST("/remove", handlers.RemoveWorktrees)
	writes.POST("/:worktree_id/exit/prepare", handlers.PrepareWorktreeExitPlan)
	writes.POST("/:worktree_id/exit/actions", handlers.RunWorktreeExitAction)
	writes.POST("/:worktree_id/exit/cancel", handlers.CancelWorktreeExitAction)
	writes.POST("/:worktree_id/cancel", handlers.CancelWorktreeCreate)
	writes.DELETE("/:worktree_id", handlers.RemoveWorktree)
	writes.POST("/:worktree_id/dismiss", handlers.DismissWorktree)
}
