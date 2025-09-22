package mcprouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	mcps := apiBase.Group("/mcps")
	{
		mcps.POST("/export", exportMCPs)
		mcps.POST("/import", importMCPs)
		mcps.GET("", listMCPs)
		mcps.GET("/:mcp_id", getMCP)
		mcps.PUT("/:mcp_id", upsertMCP)
		mcps.DELETE("/:mcp_id", deleteMCP)
	}
}
