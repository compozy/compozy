package server

import (
	"context"
	"fmt"
	"sort"
	"time"

	docs "github.com/compozy/compozy/docs"
	agentrouter "github.com/compozy/compozy/engine/agent/router"
	authrouter "github.com/compozy/compozy/engine/auth/router"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	_ "github.com/compozy/compozy/engine/infra/monitoring" // Import for swagger docs
	"github.com/compozy/compozy/engine/infra/server/appstate"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	sizemw "github.com/compozy/compozy/engine/infra/server/middleware/size"
	"github.com/compozy/compozy/engine/memory"
	memrouter "github.com/compozy/compozy/engine/memory/router"
	"github.com/compozy/compozy/engine/task"
	tkrouter "github.com/compozy/compozy/engine/task/router"
	"github.com/compozy/compozy/engine/task/services"
	toolrouter "github.com/compozy/compozy/engine/tool/router"
	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/engine/worker"
	"github.com/compozy/compozy/engine/workflow"
	wfrouter "github.com/compozy/compozy/engine/workflow/router"
	schedulerouter "github.com/compozy/compozy/engine/workflow/schedule/router"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/otel/metric"
)

func CreateHealthHandler(server *Server, version string) gin.HandlerFunc {
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

		// Include memory health if global health service is available and update status
		var memoryHealth gin.H
		if globalHealthService := memory.GetGlobalHealthService(); globalHealthService != nil {
			memoryHealth = memory.GetMemoryHealthForMainEndpoint(c.Request.Context(), globalHealthService)

			// Update overall health status if memory is unhealthy
			if memoryHealthy, exists := memoryHealth["healthy"].(bool); exists && !memoryHealthy {
				ready = false
				healthStatus = "degraded"
			}
		}

		response := gin.H{
			"status":    healthStatus,
			"version":   version,
			"ready":     ready,
			"schedules": scheduleStatus,
		}

		// Add memory health to response if available
		if memoryHealth != nil {
			response["memory"] = memoryHealth
		}
		statusCode := 200
		if !ready {
			statusCode = 503
		}
		c.JSON(statusCode, response)
	}
}

func setupSwaggerAndDocs(router *gin.Engine, prefixURL string) {
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
}

func RegisterRoutes(ctx context.Context, router *gin.Engine, state *appstate.State, server *Server) error {
	version := core.GetVersion()
	prefixURL := fmt.Sprintf("/api/%s", version)
	apiBase := router.Group(prefixURL)

	// Get configuration
	cfg := config.Get()
	// Debug log for admin key
	log := logger.FromContext(ctx)
	if cfg.Server.Auth.AdminKey.Value() != "" {
		log.Info("Admin bootstrap key is configured")
	} else {
		log.Info("No admin bootstrap key configured")
	}

	if err := attachWebhookRegistry(ctx, state); err != nil {
		return err
	}

	// Setup Swagger and documentation endpoints
	setupSwaggerAndDocs(router, prefixURL)
	var meter metric.Meter
	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		meter = server.monitoring.Meter()
	}
	if err := registerPublicWebhookRoutes(ctx, router, state, prefixURL, meter); err != nil {
		return err
	}
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
			"description": "Next-level Agentic Orchestration Platform, tasks, and tools",
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
	router.GET("/health", CreateHealthHandler(server, version))

	// Setup auth factory and manager
	authRepo := state.Store.NewAuthRepo()
	authFactory := authuc.NewFactory(authRepo)
	authManager := authmw.NewManager(authFactory, cfg)

	// Apply global authentication middleware if enabled
	// This applies to all routes under /api/v0/*
	if cfg.Server.Auth.Enabled {
		apiBase.Use(authManager.Middleware())
		apiBase.Use(authManager.RequireAuth())
	}

	// Register auth routes (they handle their own specialized auth requirements)
	// Auth routes must be registered AFTER global middleware to override with specific requirements
	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		authrouter.RegisterRoutesWithMetrics(apiBase, authFactory, server.monitoring.Meter())
	} else {
		authrouter.RegisterRoutes(apiBase, authFactory)
	}

	// Register all component routers (no auth parameter needed - handled globally)
	wfrouter.Register(apiBase)
	tkrouter.Register(apiBase)
	agentrouter.Register(apiBase)
	toolrouter.Register(apiBase)
	schedulerouter.Register(apiBase)
	memrouter.Register(apiBase)

	// Register memory health routes if global health service is available
	if globalHealthService := memory.GetGlobalHealthService(); globalHealthService != nil {
		memory.RegisterMemoryHealthRoutes(apiBase, globalHealthService)
	}

	log.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
		"swagger_base_path", docs.SwaggerInfo.BasePath,
		"auth_enabled", cfg.Server.Auth.Enabled,
	)
	return nil
}

func registerPublicWebhookRoutes(
	ctx context.Context,
	router *gin.Engine,
	state *appstate.State,
	prefixURL string,
	meter metric.Meter,
) error {
	const defaultMaxBody int64 = 1 << 20
	hooks := router.Group(prefixURL + "/hooks")
	hooks.Use(sizemw.BodySizeLimiter(defaultMaxBody))
	var reg webhook.Lookup
	if ext, ok := state.Extensions[appstate.WebhookRegistryKey]; ok {
		if r, ok := ext.(webhook.Lookup); ok {
			reg = r
		}
	}
	if reg == nil {
		reg = webhook.NewRegistry()
	}
	eval, err := task.NewCELEvaluator()
	if err != nil {
		return fmt.Errorf("failed to init CEL evaluator: %w", err)
	}
	filter := webhook.NewCELAdapter(eval)
	var disp services.SignalDispatcher
	if state.Worker != nil {
		disp = worker.NewSignalDispatcher(
			state.Worker.GetClient(),
			state.Worker.GetDispatcherID(),
			state.Worker.GetTaskQueue(),
			state.Worker.GetDispatcherID(),
		)
	}
	orchestrator := webhook.NewOrchestrator(reg, filter, disp, nil, defaultMaxBody, 10*time.Minute)
	if meter != nil {
		orchestrator.SetMetrics(webhook.NewMetrics(ctx, meter))
	}
	webhook.RegisterPublic(hooks, orchestrator)
	_ = ctx
	return nil
}

func attachWebhookRegistry(ctx context.Context, state *appstate.State) error {
	reg := webhook.NewRegistry()
	for _, wf := range state.Workflows {
		for i := range wf.Triggers {
			t := wf.Triggers[i]
			if t.Type == workflow.TriggerTypeWebhook && t.Webhook != nil {
				entry := webhook.RegistryEntry{WorkflowID: wf.ID, Webhook: t.Webhook}
				if err := reg.Add(t.Webhook.Slug, entry); err != nil {
					return fmt.Errorf(
						"webhook registry: failed to add slug '%s' from workflow '%s': %w",
						t.Webhook.Slug,
						wf.ID,
						err,
					)
				}
			}
		}
	}
	state.Extensions[appstate.WebhookRegistryKey] = reg
	slugs := reg.Slugs()
	limit := min(len(slugs), 5)
	log := logger.FromContext(ctx)
	if limit > 0 {
		sort.Strings(slugs)
		log.Info("Webhook registry initialized", "count", len(slugs), "slugs", slugs[:limit])
	} else {
		log.Info("Webhook registry initialized", "count", 0)
	}
	return nil
}
