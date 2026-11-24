package httpapi

import (
	"github.com/compozy/compozy/internal/api/core"
	"github.com/gin-gonic/gin"
)

func registerAccountRoutes(api gin.IRouter, handlers *Handlers, signIn *core.SignInRoutes) {
	routes := api.Group("/account", handlers.privilegedMutationGuard())
	core.RegisterAccountRoutes(routes, handlers.BaseHandlers, signIn)
}
