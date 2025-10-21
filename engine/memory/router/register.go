package memrouter

import (
	"github.com/gin-gonic/gin"
)

// Register wires memory routes under /memory.
// Note: Authentication is enforced globally via server middleware in engine/infra/server/register.go
// when cfg.Server.Auth.Enabled is true. All memory endpoints are protected by the global auth middleware.
func Register(apiBase *gin.RouterGroup) {
	memoryGroup := apiBase.Group("/memory")
	refGroup := memoryGroup.Group("/:memory_ref")
	refGroup.Use(ExtractMemoryContext())
	{
		refGroup.GET("/read", readMemory)

		refGroup.POST("/write", writeMemory)

		refGroup.POST("/append", appendMemory)

		refGroup.POST("/delete", deleteMemory)

		refGroup.POST("/flush", flushMemory)

		refGroup.GET("/health", healthMemory)

		refGroup.POST("/clear", clearMemory)

		refGroup.GET("/stats", statsMemory)
	}
}
