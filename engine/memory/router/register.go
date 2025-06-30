package memrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	// TODO: CRITICAL SECURITY - Add authentication middleware before production use
	// All memory routes are currently PUBLIC and accessible without authentication
	// This must be addressed before deployment to any environment with sensitive data

	// Memory routes
	memoryGroup := apiBase.Group("/memory")

	// Routes with memory reference only (key moved to query params or body)
	refGroup := memoryGroup.Group("/:memory_ref")
	refGroup.Use(ExtractMemoryContext())
	{
		// GET /api/v0/memory/:memory_ref/read?key={key}
		// Read memory content
		refGroup.GET("/read", readMemory)

		// POST /api/v0/memory/:memory_ref/write
		// Write/replace memory content (key in body)
		refGroup.POST("/write", writeMemory)

		// POST /api/v0/memory/:memory_ref/append
		// Append to memory (key in body)
		refGroup.POST("/append", appendMemory)

		// POST /api/v0/memory/:memory_ref/delete
		// Delete memory (key in body)
		refGroup.POST("/delete", deleteMemory)

		// POST /api/v0/memory/:memory_ref/flush
		// Flush memory (key in body)
		refGroup.POST("/flush", flushMemory)

		// GET /api/v0/memory/:memory_ref/health?key={key}
		// Get memory health
		refGroup.GET("/health", healthMemory)

		// POST /api/v0/memory/:memory_ref/clear
		// Clear memory (key in body)
		refGroup.POST("/clear", clearMemory)

		// GET /api/v0/memory/:memory_ref/stats?key={key}
		// Get memory statistics
		refGroup.GET("/stats", statsMemory)
	}
}
