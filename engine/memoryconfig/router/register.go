package memoryrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	memories := apiBase.Group("/memories")
	{
		memories.GET("", listMemories)
		memories.GET("/:memory_id", getMemory)
		memories.PUT("/:memory_id", upsertMemory)
		memories.DELETE("/:memory_id", deleteMemory)
	}
}
