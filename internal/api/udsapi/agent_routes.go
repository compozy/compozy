package udsapi

import "github.com/gin-gonic/gin"

func registerAgentRoutes(api gin.IRouter, handlers *Handlers) {
	agents := api.Group("/agents")
	{
		agents.GET("", handlers.ListAgents)
		agents.GET("/catalog", handlers.ListAgentCatalog)
		agents.POST("", handlers.CreateAgent)
		agents.PUT("/:name", handlers.UpdateAgent)
		agents.DELETE("/:name", handlers.DeleteAgent)
		agents.POST("/:name/duplicate", handlers.DuplicateAgent)
		agents.GET("/:name/soul", handlers.GetAgentSoul)
		agents.POST("/:name/soul/validate", handlers.ValidateAgentSoulDefinition)
		agents.PUT("/:name/soul", handlers.PutAgentSoul)
		agents.DELETE("/:name/soul", handlers.DeleteAgentSoul)
		agents.GET("/:name/soul/history", handlers.ListAgentSoulHistory)
		agents.POST("/:name/soul/rollback", handlers.RollbackAgentSoul)
		agents.GET("/:name/heartbeat", handlers.GetAgentHeartbeat)
		agents.POST("/:name/heartbeat/validate", handlers.ValidateAgentHeartbeat)
		agents.PUT("/:name/heartbeat", handlers.PutAgentHeartbeat)
		agents.DELETE("/:name/heartbeat", handlers.DeleteAgentHeartbeat)
		agents.GET("/:name/heartbeat/history", handlers.ListAgentHeartbeatHistory)
		agents.POST("/:name/heartbeat/rollback", handlers.RollbackAgentHeartbeat)
		agents.GET("/:name/heartbeat/status", handlers.GetAgentHeartbeatStatus)
		agents.POST("/:name/heartbeat/wake", handlers.WakeAgentHeartbeat)
		agents.GET("/:name", handlers.GetAgent)
	}
}
