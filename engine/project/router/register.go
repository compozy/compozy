package projectrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	project := apiBase.Group("/project")
	{
		project.GET("", getProject)
		project.PUT("", upsertProject)
		project.DELETE("", deleteProject)
	}
}
