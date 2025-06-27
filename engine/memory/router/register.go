package memrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	// TODO: CRITICAL SECURITY - Add authentication middleware before production use
	// All memory routes are currently PUBLIC and accessible without authentication
	// This must be addressed before deployment to any environment with sensitive data

	// Memory routes
	memoryGroup := apiBase.Group("/memory")

	// Routes with memory reference and key
	refGroup := memoryGroup.Group("/:memory_ref/:key")
	refGroup.Use(ExtractMemoryContext())
	{
		// GET /api/v0/memory/:memory_ref/:key
		// Read memory content
		refGroup.GET("", readMemory)

		// PUT /api/v0/memory/:memory_ref/:key
		// Write/replace memory content
		refGroup.PUT("", writeMemory)

		// POST /api/v0/memory/:memory_ref/:key
		// Append to memory
		refGroup.POST("", appendMemory)

		// DELETE /api/v0/memory/:memory_ref/:key
		// Delete memory
		refGroup.DELETE("", deleteMemory)

		// POST /api/v0/memory/:memory_ref/:key/flush
		// Flush memory
		refGroup.POST("/flush", flushMemory)

		// GET /api/v0/memory/:memory_ref/:key/health
		// Get memory health
		refGroup.GET("/health", healthMemory)

		// POST /api/v0/memory/:memory_ref/:key/clear
		// Clear memory
		refGroup.POST("/clear", clearMemory)

		// GET /api/v0/memory/:memory_ref/:key/stats
		// Get memory statistics
		refGroup.GET("/stats", statsMemory)
	}
}
