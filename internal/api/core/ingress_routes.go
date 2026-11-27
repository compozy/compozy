package core

import "github.com/gin-gonic/gin"

func RegisterIngressRoutes(api gin.IRouter, handlers *BaseHandlers, guards ...gin.HandlerFunc) {
	routes := api.Group("/ingress/bindings", guards...)
	routes.POST("", handlers.BindIngress)
	routes.DELETE("/:subject_kind/:subject_id", handlers.UnbindIngress)
}
