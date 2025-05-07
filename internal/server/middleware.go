package server

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggerMiddleware returns a Gin middleware for request logging
func LoggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(params gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s\n",
			params.TimeStamp.Format(time.RFC3339),
			params.Method,
			params.Path,
			params.StatusCode,
			params.Latency,
		)
	})
}

// CORSMiddleware returns a Gin middleware for CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// AppStateMiddleware adds the app state to the request context
func AppStateMiddleware(state *AppState) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := WithAppState(c.Request.Context(), state)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
