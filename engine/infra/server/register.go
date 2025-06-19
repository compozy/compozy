package server

import (
	"context"
	"fmt"

	docs "github.com/compozy/compozy/docs"
	agentrouter "github.com/compozy/compozy/engine/agent/router"
	"github.com/compozy/compozy/engine/core"
	_ "github.com/compozy/compozy/engine/infra/monitoring" // Import for swagger docs
	"github.com/compozy/compozy/engine/infra/server/appstate"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	schedulerouter "github.com/compozy/compozy/engine/workflow/schedule/router"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func createHealthHandler(server *Server, version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ready := true
		healthStatus := "healthy"
		// Include schedule reconciliation status if available
		scheduleStatus := gin.H{
			"reconciled": true,
			"status":     "ready",
		}
		if server != nil {
			completed, lastAttempt, attemptCount, lastError := server.GetReconciliationStatus()
			scheduleStatus = gin.H{
				"reconciled":    completed,
				"last_attempt":  lastAttempt,
				"attempt_count": attemptCount,
			}
			switch {
			case completed:
				scheduleStatus["status"] = "ready"
			case lastError != nil:
				// Log the detailed error for internal diagnostics
				logger.FromContext(c).
					Warn("Readiness probe check failed due to reconciliation error", "error", lastError)
				scheduleStatus["status"] = "retrying"
				scheduleStatus["last_error"] = "reconciliation failed, see server logs for details"
				ready = false
				healthStatus = "not_ready"
			default:
				scheduleStatus["status"] = "initializing"
				ready = false
				healthStatus = "not_ready"
			}
		}
		response := gin.H{
			"status":    healthStatus,
			"version":   version,
			"ready":     ready,
			"schedules": scheduleStatus,
		}
		statusCode := 200
		if !ready {
			statusCode = 503
		}
		c.JSON(statusCode, response)
	}
}

func RegisterRoutes(ctx context.Context, router *gin.Engine, state *appstate.State, server *Server) error {
	version := core.GetVersion()
	prefixURL := fmt.Sprintf("/api/%s", version)
	apiBase := router.Group(prefixURL)

	// Configure Swagger Info
	docs.SwaggerInfo.BasePath = prefixURL
	docs.SwaggerInfo.Host = ""
	docs.SwaggerInfo.Schemes = []string{"http", "https"}

	// Configure gin-swagger with custom URL
	url := ginSwagger.URL("/swagger/doc.json")
	router.GET("/swagger-ui", func(c *gin.Context) {
		c.Redirect(301, "/swagger/index.html")
	})
	router.GET("/docs-ui", func(c *gin.Context) {
		c.Redirect(301, "/docs/index.html")
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		url,
		ginSwagger.DefaultModelsExpandDepth(-1),
	))
	router.GET("/docs/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		url,
		ginSwagger.DefaultModelsExpandDepth(-1),
	))

	// Root endpoint with API information
	router.GET("/", func(c *gin.Context) {
		host := c.Request.Host
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, host)

		c.JSON(200, gin.H{
			"name":        "Compozy API",
			"version":     version,
			"description": "Workflow orchestration engine for AI agents, tasks, and tools",
			"endpoints": gin.H{
				"health":  fmt.Sprintf("%s/health", baseURL),
				"api":     fmt.Sprintf("%s%s", baseURL, prefixURL),
				"swagger": fmt.Sprintf("%s/swagger/index.html", baseURL),
				"docs":    fmt.Sprintf("%s/docs/index.html", baseURL),
				"openapi": fmt.Sprintf("%s/swagger/doc.json", baseURL),
			},
		})
	})

	// Health check endpoint with readiness probe
	router.GET("/health", createHealthHandler(server, version))

	// Register all component routers
	wfrouter.Register(apiBase)
	tkrouter.Register(apiBase)
	agentrouter.Register(apiBase)
	toolrouter.Register(apiBase)
	schedulerouter.Register(apiBase)

	log := logger.FromContext(ctx)
	log.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
		"swagger_base_path", docs.SwaggerInfo.BasePath,
	)
	return nil
}
