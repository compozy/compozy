package memrouter

import (
	"github.com/compozy/compozy/engine/auth/uc"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/gin-gonic/gin"
)

func Register(apiBase *gin.RouterGroup, authFactory *uc.Factory) {
	// Create auth middleware manager
	authManager := authmw.NewManager(authFactory)

	// Memory routes - all routes require authentication
	memoryGroup := apiBase.Group("/memory")
	memoryGroup.Use(authManager.RequireAuth())

	// Routes with memory reference only (key moved to query params or body)
	refGroup := memoryGroup.Group("/:memory_ref")
	refGroup.Use(ExtractMemoryContext())
	{
		// GET /memory/:memory_ref/read?key={key}
		// Read memory content
		refGroup.GET("/read", readMemory)

		// POST /memory/:memory_ref/write
		// Write/replace memory content (key in body)
		refGroup.POST("/write", writeMemory)

		// POST /memory/:memory_ref/append
		// Append to memory (key in body)
		refGroup.POST("/append", appendMemory)

		// POST /memory/:memory_ref/delete
		// Delete memory (key in body)
		refGroup.POST("/delete", deleteMemory)

		// POST /memory/:memory_ref/flush
		// Flush memory (key in body)
		refGroup.POST("/flush", flushMemory)

		// GET /memory/:memory_ref/health?key={key}
		// Get memory health
		refGroup.GET("/health", healthMemory)

		// POST /memory/:memory_ref/clear
		// Clear memory (key in body)
		refGroup.POST("/clear", clearMemory)

		// GET /memory/:memory_ref/stats?key={key}
		// Get memory statistics
		refGroup.GET("/stats", statsMemory)
	}
}
