package router

import (
	"github.com/compozy/compozy/engine/auth/uc"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/metric"
)

// RegisterRoutes registers all auth routes
func RegisterRoutes(apiBase *gin.RouterGroup, factory *uc.Factory) {
	RegisterRoutesWithMetrics(apiBase, factory, nil)
}

// RegisterRoutesWithMetrics registers all auth routes with metrics instrumentation
func RegisterRoutesWithMetrics(apiBase *gin.RouterGroup, factory *uc.Factory, meter metric.Meter) {
	handler := NewHandler(factory)
	authManager := authmw.NewManager(factory)

	// Add metrics instrumentation if meter is provided
	if meter != nil {
		authManager = authManager.WithMetrics(meter)
	}

	// Auth endpoints (require authentication)
	auth := apiBase.Group("/auth")
	{
		// These endpoints require authentication
		auth.Use(authManager.Middleware())
		auth.Use(authManager.RequireAuth())
		auth.POST("/generate", handler.GenerateKey)
		auth.GET("/keys", handler.ListKeys)
		auth.DELETE("/keys/:id", handler.RevokeKey)
	}

	// Admin endpoints for user management
	admin := apiBase.Group("/users")
	admin.Use(authManager.Middleware())
	admin.Use(authManager.RequireAdmin())
	{
		admin.GET("", handler.ListUsers)
		admin.POST("", handler.CreateUser)
		admin.PATCH("/:id", handler.UpdateUser)
		admin.DELETE("/:id", handler.DeleteUser)
	}
}
