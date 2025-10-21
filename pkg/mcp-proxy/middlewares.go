package mcpproxy

import (
	"net/http"
	"runtime/debug"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// wrapWithGinMiddlewares wraps an http.Handler with gin middlewares
func wrapWithGinMiddlewares(handler http.Handler, middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	engine := gin.New()
	for _, middleware := range middlewares {
		engine.Use(middleware)
	}
	engine.Use(func(c *gin.Context) {
		if handler == nil {
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{"error": "Handler not initialized", "details": "Handler not initialized"},
			)
			return
		}
		handler.ServeHTTP(c.Writer, c.Request)
	})
	return func(c *gin.Context) {
		engine.HandleContext(c)
	}
}

// recoverMiddleware creates a panic recovery middleware
func recoverMiddleware(clientName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.FromContext(c.Request.Context())
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic recovered in MCP proxy",
					"client", clientName,
					"panic", r,
					"stack", string(debug.Stack()))
				c.AbortWithStatusJSON(
					http.StatusInternalServerError,
					gin.H{"error": "Internal server error", "details": "An unexpected error occurred"},
				)
				c.Abort()
			}
		}()
		c.Next()
	}
}
