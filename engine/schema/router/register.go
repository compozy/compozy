package schemarouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	schemas := apiBase.Group("/schemas")
	{
		schemas.POST("/export", exportSchemas)
		schemas.POST("/import", importSchemas)
		schemas.GET("", listSchemas)
		schemas.GET("/:schema_id", getSchema)
		schemas.PUT("/:schema_id", upsertSchema)
		schemas.DELETE("/:schema_id", deleteSchema)
	}
}
