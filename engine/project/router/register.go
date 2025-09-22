package projectrouter

import "github.com/gin-gonic/gin"

func Register(apiBase *gin.RouterGroup) {
	project := apiBase.Group("/project")
	{
		project.POST("/export", exportProject)
		project.POST("/import", importProject)
		project.GET("", getProject)
		project.PUT("", upsertProject)
		project.DELETE("", deleteProject)
	}
}
