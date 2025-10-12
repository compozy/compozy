package knowledgerouter

import "github.com/gin-gonic/gin"

func Register(api *gin.RouterGroup) {
	group := api.Group("/knowledge-bases")
	{
		group.GET("", listKnowledgeBases)
		group.GET("/:kb_id", getKnowledgeBase)
		group.PUT("/:kb_id", upsertKnowledgeBase)
		group.DELETE("/:kb_id", deleteKnowledgeBase)
		group.POST("/:kb_id/ingest", ingestKnowledgeBase)
		group.POST("/:kb_id/query", queryKnowledgeBase)
	}
}
