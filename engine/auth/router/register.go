package router

import (
	"context"

	"github.com/compozy/compozy/engine/auth/uc"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/metric"
)

// RegisterRoutes registers all auth routes
func RegisterRoutes(apiBase *gin.RouterGroup, factory *uc.Factory, cfg *config.Config) {
	RegisterRoutesWithMetrics(context.Background(), apiBase, factory, cfg, nil)
}

// RegisterRoutesWithMetrics registers all auth routes with metrics instrumentation
func RegisterRoutesWithMetrics(
	ctx context.Context,
	apiBase *gin.RouterGroup,
	factory *uc.Factory,
	cfg *config.Config,
	meter metric.Meter,
) {
	handler := NewHandler(factory)
	authManager := authmw.NewManager(factory, cfg)
	// Add metrics instrumentation if meter is provided
	if meter != nil {
		authManager = authManager.WithMetrics(ctx, meter)
	}
	// Auth endpoints
	auth := apiBase.Group("/auth")
	{
		auth.POST("/generate", handler.GenerateKey)
		auth.GET("/keys", handler.ListKeys)
		auth.DELETE("/keys/:id", handler.RevokeKey)
	}
	// Admin endpoints for user management
	admin := apiBase.Group("/users")
	admin.Use(authManager.RequireAdmin())
	{
		admin.GET("", handler.ListUsers)
		admin.POST("", handler.CreateUser)
		admin.PATCH("/:id", handler.UpdateUser)
		admin.DELETE("/:id", handler.DeleteUser)
	}
}
