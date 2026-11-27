package udsapi

import (
	"github.com/compozy/compozy/internal/api/core"
	"github.com/gin-gonic/gin"
)

func registerAccountRoutes(api gin.IRouter, handlers *Handlers, signIn *core.SignInRoutes) {
	routes := api.Group("/account")
	core.RegisterAccountRoutes(routes, handlers.BaseHandlers, signIn)
}
