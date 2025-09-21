package modelrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	models := apiBase.Group("/models")
	{
		models.GET("", listModels)
		models.GET("/:model_id", getModel)
		models.PUT("/:model_id", upsertModel)
		models.DELETE("/:model_id", deleteModel)
	}
}
