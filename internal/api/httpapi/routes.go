package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

// RegisterRoutes registers the shared daemon API routes on the supplied router.
func RegisterRoutes(router gin.IRouter, handlers *core.Handlers, noRoute ...gin.HandlerFunc) {
	core.RegisterRoutes(router, handlers)
	if engine, ok := router.(*gin.Engine); ok && len(noRoute) > 0 && noRoute[0] != nil {
		engine.NoRoute(noRoute[0])
	}
}
