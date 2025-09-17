package prmiddleware

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/gin-gonic/gin"
)

// Middleware injects the project name into the request context.
func Middleware(projectName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := core.WithProjectName(c.Request.Context(), projectName)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
