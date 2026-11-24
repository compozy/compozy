package udsapi

import "github.com/gin-gonic/gin"

func registerAccountRoutes(api gin.IRouter, handlers *Handlers) {
	routes := api.Group("/account")
	routes.GET("", handlers.GetAccount)
	routes.POST("/login", handlers.StartAccountLogin)
	routes.DELETE("/login", handlers.CancelAccountLogin)
	routes.GET("/sign-in/events", handlers.StreamAccountSignIn)
	routes.POST("/logout", handlers.LogoutAccount)
	routes.GET("/stream", handlers.StreamAccount)
	routes.GET("/devices", handlers.ListAccountDevices)
	routes.PATCH("/devices/:id", handlers.PatchAccountDevice)
	routes.DELETE("/devices/:id", handlers.RevokeAccountDevice)
	routes.GET("/approvals", handlers.ListAccountApprovals)
	routes.GET("/approvals/:id", handlers.GetAccountApproval)
	routes.POST("/approvals/:id/decision", handlers.DecideAccountApproval)
	routes.POST("/checkout", handlers.CheckoutAccount)
	routes.POST("/billing-portal", handlers.AccountBillingPortal)
}
