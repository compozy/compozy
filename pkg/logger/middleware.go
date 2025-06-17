package logger

import (
	"github.com/gin-gonic/gin"
)

func Middleware(log Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add logger to Gin context
		ctx := ContextWithLogger(c.Request.Context(), log)
		c.Request = c.Request.WithContext(ctx)
		ctxLog := FromContext(c.Request.Context())
		ctxLog.Info("request", "path", c.Request.URL.Path)
		c.Next()
	}
}
