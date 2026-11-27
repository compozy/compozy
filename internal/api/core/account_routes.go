package core

import "github.com/gin-gonic/gin"

func RegisterAccountRoutes(routes gin.IRouter, handlers *BaseHandlers, signIn *SignInRoutes) {
	signIn.RequireSignedIn(SignInAnonymous, func() {
		routes.GET("", handlers.GetAccount)
		routes.POST("/login", handlers.StartAccountLogin)
		routes.DELETE("/login", handlers.CancelAccountLogin)
		routes.GET("/sign-in/events", handlers.StreamAccountSignIn)
		routes.POST("/logout", handlers.LogoutAccount)
	})
	signIn.RequireSignedIn(SignInAccount, func() {
		routes.PATCH("", handlers.PatchAccountEmail)
		routes.DELETE("", handlers.DeleteAccount)
		routes.POST("/sign-in-notices/dismiss", handlers.DismissAccountSignInNotices)
		routes.GET("/tokens", handlers.ListAccountTokens)
		routes.POST("/tokens", handlers.CreateAccountToken)
		routes.DELETE("/tokens/:id", handlers.RevokeAccountToken)
		routes.GET("/stream", handlers.StreamAccount)
		routes.GET("/devices", handlers.ListAccountDevices)
		routes.PATCH("/devices/:id", handlers.PatchAccountDevice)
		routes.DELETE("/devices/:id", handlers.RevokeAccountDevice)
		routes.GET("/approvals", handlers.ListAccountApprovals)
		routes.GET("/approvals/:id", handlers.GetAccountApproval)
		routes.POST("/approvals/:id/decision", handlers.DecideAccountApproval)
		routes.POST("/checkout", handlers.CheckoutAccount)
		routes.POST("/billing-portal", handlers.AccountBillingPortal)
	})
}
