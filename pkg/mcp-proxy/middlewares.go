package mcpproxy

import (
	"net/http"
	"runtime/debug"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// wrapWithGinMiddlewares wraps an http.Handler with gin middlewares
func wrapWithGinMiddlewares(handler http.Handler, middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	// Create a new gin engine to properly chain middlewares
	engine := gin.New()

	// Add all middlewares to the engine
	for _, middleware := range middlewares {
		engine.Use(middleware)
	}

	// Add the final handler that calls the wrapped http.Handler
	engine.Use(func(c *gin.Context) {
		if handler == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Handler not initialized"})
			return
		}
		handler.ServeHTTP(c.Writer, c.Request)
	})

	// Return a handler that uses the engine
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				c.Abort()
			}
		}()
		c.Next()
	}
}
