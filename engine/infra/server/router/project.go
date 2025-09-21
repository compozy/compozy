package router

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func ProjectFromQueryOrDefault(c *gin.Context) string {
	project := strings.TrimSpace(c.Query("project"))
	if project != "" {
		return project
	}
	state := GetAppState(c)
	if state == nil {
		c.Abort()
		return ""
	}
	return state.ProjectConfig.Name
}
