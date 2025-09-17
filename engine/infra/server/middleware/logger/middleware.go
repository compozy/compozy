package lgmiddleware

import (
	"context"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Middleware attaches request-scoped logger metadata.
func Middleware(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		mgr := config.ManagerFromContext(ctx)
		log := logger.FromContext(ctx)
		reqCtx := c.Request.Context()
		reqCtx = config.ContextWithManager(reqCtx, mgr)
		reqCtx = logger.ContextWithLogger(reqCtx, log)
		c.Request = c.Request.WithContext(reqCtx)
		c.Next()
	}
}
