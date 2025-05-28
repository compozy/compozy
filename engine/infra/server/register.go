package server

import (
	"fmt"

	docs "github.com/compozy/compozy/docs"
	agentrouter "github.com/compozy/compozy/engine/agent/router"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/appstate"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func RegisterRoutes(router *gin.Engine, state *appstate.State) error {
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

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"version": version,
		})
	})

	// Register all component routers
	wfrouter.Register(apiBase)
	tkrouter.Register(apiBase)
	agentrouter.Register(apiBase)
	toolrouter.Register(apiBase)

	logger.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
		"swagger_base_path", docs.SwaggerInfo.BasePath,
	)
	return nil
}
