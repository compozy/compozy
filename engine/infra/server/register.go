package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	docs "github.com/compozy/compozy/docs"
	agentrouter "github.com/compozy/compozy/engine/agent/router"
	authrouter "github.com/compozy/compozy/engine/auth/router"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	_ "github.com/compozy/compozy/engine/infra/monitoring" // Import for swagger docs
	"github.com/compozy/compozy/engine/infra/server/appstate"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	sizemw "github.com/compozy/compozy/engine/infra/server/middleware/size"
	"github.com/compozy/compozy/engine/infra/server/routes"
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

// CreateHealthHandler creates a health check endpoint handler
func CreateHealthHandler(server *Server, version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ready, healthStatus, scheduleStatus := buildScheduleStatus(ctx, server)
		temporalReady := false
		workerReady := false
		mcpReady := false
		if server != nil {
			temporalReady = server.isTemporalReady()
			workerReady = server.isWorkerReady()
			mcpReady = server.isMCPReady()
		}
		if server != nil && !server.isFullyReady() {
			ready = false
			healthStatus = statusNotReady
		}
		memoryHealth := buildMemoryHealth(ctx, &ready, &healthStatus)
		response := buildHealthResponse(healthStatus, version, ready, scheduleStatus, memoryHealth)
		// Enrich with per-component readiness (align with /readyz) for a single aggregated view
		response["temporal"] = gin.H{"ready": temporalReady}
		response["worker"] = gin.H{"running": workerReady}
		response["mcp_proxy"] = gin.H{"ready": mcpReady}
		statusCode := determineHealthStatusCode(ready)

		c.JSON(statusCode, gin.H{
			"data":    response,
			"message": "Success",
		})
	}
}

// setupSwaggerAndDocs configures Swagger UI and API documentation routes
func setupSwaggerAndDocs(router *gin.Engine, prefixURL string) {
	// Configure Swagger Info
	docs.SwaggerInfo.BasePath = prefixURL
	docs.SwaggerInfo.Host = ""
	docs.SwaggerInfo.Schemes = []string{"http", "https"}
	// Configure gin-swagger with custom URL
	url := ginSwagger.URL("/swagger/doc.json")
	router.GET("/swagger-ui", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	router.GET("/docs-ui", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
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

// RegisterRoutes orchestrates the complete setup of all HTTP routes
func RegisterRoutes(ctx context.Context, router *gin.Engine, state *appstate.State, server *Server) error {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing config in context; ensure config.ContextWithManager is set before server init")
	}
	version, prefixURL, _ := setupBasicConfiguration(ctx)
	apiBase := router.Group(prefixURL)

	if err := setupWebhookSystem(ctx, state, router, server); err != nil {
		return err
	}

	setupDiagnosticEndpoints(router, version, prefixURL, server)

	if err := setupAuthSystem(ctx, apiBase, state, cfg, server); err != nil {
		return err
	}

	setupComponentRoutes(apiBase, server)

	logRegistrationComplete(ctx, state, cfg)
	return nil
}

// registerPublicWebhookRoutes sets up public webhook endpoints with middleware and orchestration
func registerPublicWebhookRoutes(
	ctx context.Context,
	router *gin.Engine,
	state *appstate.State,
	server *Server,
	meter metric.Meter,
) error {
	cfg := config.FromContext(ctx)
	if cfg == nil {
		return fmt.Errorf("missing config in context")
	}
	limiterMax := cfg.Webhooks.DefaultMaxBody
	hooks := router.Group(routes.Hooks())
	hooks.Use(sizemw.BodySizeLimiter(limiterMax))
	// Ensure project name is present in request context for downstream dispatch
	if state.ProjectConfig.Name == "" {
		return fmt.Errorf("project name is empty; set project.projectConfig.name")
	}
	hooks.Use(ProjectContextMiddleware(state.ProjectConfig.Name))
	var reg webhook.Lookup
	if ext, ok := state.WebhookRegistry(); ok {
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
	var dispatcher services.SignalDispatcher
	if state.Worker != nil {
		dispatcherID := state.Worker.GetDispatcherID()
		taskQueue := state.Worker.GetTaskQueue()
		if dispatcherID != "" && taskQueue != "" {
			dispatcher = worker.NewSignalDispatcher(
				state.Worker.GetClient(),
				dispatcherID,
				taskQueue,
				state.Worker.GetServerID(),
			)
		} else {
			logger.FromContext(ctx).Warn(
				"Worker missing dispatcher metadata; webhook dispatch disabled",
				"dispatcher_id", dispatcherID,
				"task_queue", taskQueue,
			)
		}
	}
	// Pass zeroes so NewOrchestrator falls back to config values internally.
	// Wire idempotency service: use Redis when configured, otherwise in-memory.
	var idemSvc webhook.Service
	if server != nil && server.redisClient != nil {
		// server.redisClient (*redis.Client) satisfies cache.RedisInterface
		svc, err := webhook.NewServiceFromCache(server.redisClient)
		if err != nil {
			return fmt.Errorf("failed to initialize redis idempotency service: %w", err)
		}
		idemSvc = svc
	} else {
		idemSvc = webhook.NewInMemoryService()
	}
	orchestrator := webhook.NewOrchestrator(cfg, reg, filter, dispatcher, idemSvc, 0, 0)
	if meter != nil {
		metrics, err := webhook.NewMetrics(ctx, meter)
		if err != nil {
			return fmt.Errorf("failed to initialize webhook metrics: %w", err)
		}
		orchestrator.SetMetrics(metrics)
	}
	webhook.RegisterPublic(hooks, orchestrator)
	logger.FromContext(ctx).Info("Public webhook routes registered", "path", routes.Hooks())
	return nil
}

// attachWebhookRegistry builds webhook registry from workflow triggers
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
	state.SetWebhookRegistry(reg)
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

// setupBasicConfiguration initializes version, URL prefix, and configuration
func setupBasicConfiguration(ctx context.Context) (string, string, *config.Config) {
	version := core.GetVersion()
	prefixURL := routes.Base()
	cfg := config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if cfg.Server.Auth.AdminKey.Value() != "" {
		log.Info("Admin bootstrap key is configured")
	} else {
		log.Info("No admin bootstrap key configured")
	}
	return version, prefixURL, cfg
}

// setupComponentRoutes registers all component-specific API routes
func setupComponentRoutes(apiBase *gin.RouterGroup, _ *Server) {
	wfrouter.Register(apiBase)
	tkrouter.Register(apiBase)
	agentrouter.Register(apiBase)
	toolrouter.Register(apiBase)
	schedulerouter.Register(apiBase)
	memrouter.Register(apiBase)

	if globalHealthService := memory.GetGlobalHealthService(); globalHealthService != nil {
		memory.RegisterMemoryHealthRoutes(apiBase, globalHealthService)
	}
}

// setupWebhookSystem initializes webhook registry and routes with monitoring
func setupWebhookSystem(ctx context.Context, state *appstate.State, router *gin.Engine, server *Server) error {
	if err := attachWebhookRegistry(ctx, state); err != nil {
		return err
	}

	setupSwaggerAndDocs(router, routes.Base())

	var meter metric.Meter
	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		meter = server.monitoring.Meter()
	}

	return registerPublicWebhookRoutes(ctx, router, state, server, meter)
}

// setupDiagnosticEndpoints configures root and health check endpoints
func setupDiagnosticEndpoints(router *gin.Engine, version, prefixURL string, server *Server) {
	// Root endpoint with API information
	router.GET("/", createRootHandler(version, prefixURL))
	// Health check endpoint with readiness probe
	router.GET("/health", CreateHealthHandler(server, version))
	// MCP health: reports embedded MCP proxy readiness in standalone mode
	router.GET("/mcp/health", func(c *gin.Context) {
		ready := false
		if server != nil {
			ready = server.isMCPReady()
		}
		code := determineHealthStatusCode(ready)
		st := "ok"
		if !ready {
			st = statusNotReady
		}
		c.JSON(code, gin.H{"status": st})
	})
	// Kubernetes-style liveness and readiness endpoints
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx := c.Request.Context()
		ready, healthStatus, scheduleStatus := buildScheduleStatus(ctx, server)
		temporalReady := false
		workerReady := false
		mcpReady := false
		if server != nil {
			temporalReady = server.isTemporalReady()
			workerReady = server.isWorkerReady()
			mcpReady = server.isMCPReady()
			if !server.isFullyReady() {
				ready = false
				healthStatus = statusNotReady
			}
		} else {
			ready = false
			healthStatus = statusNotReady
		}
		statusCode := determineHealthStatusCode(ready)
		c.JSON(statusCode, gin.H{
			"data": gin.H{
				"status":    healthStatus,
				"version":   version,
				"ready":     ready,
				"temporal":  gin.H{"ready": temporalReady},
				"worker":    gin.H{"running": workerReady},
				"mcp_proxy": gin.H{"ready": mcpReady},
				"schedules": scheduleStatus,
			},
			"message": "Success",
		})
	})
}

// createRootHandler creates the root endpoint handler with API information
func createRootHandler(version, prefixURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, host)

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
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
			},
			"message": "Success",
		})
	}
}

// setupAuthSystem configures authentication middleware and routes
func setupAuthSystem(
	ctx context.Context,
	apiBase *gin.RouterGroup,
	state *appstate.State,
	cfg *config.Config,
	server *Server,
) error {
	authRepo := state.Store.NewAuthRepo()
	authFactory := authuc.NewFactory(authRepo)
	authManager := authmw.NewManager(authFactory, cfg)

	if cfg.Server.Auth.Enabled {
		apiBase.Use(authManager.Middleware())
		apiBase.Use(authManager.RequireAuth())
	}

	if server != nil && server.monitoring != nil && server.monitoring.IsInitialized() {
		authrouter.RegisterRoutesWithMetrics(ctx, apiBase, authFactory, cfg, server.monitoring.Meter())
	} else {
		authrouter.RegisterRoutes(apiBase, authFactory, cfg)
	}

	return nil
}

// logRegistrationComplete logs completion of route registration process
func logRegistrationComplete(ctx context.Context, state *appstate.State, cfg *config.Config) {
	log := logger.FromContext(ctx)
	log.Info("Completed route registration",
		"total_workflows", len(state.Workflows),
		"swagger_base_path", docs.SwaggerInfo.BasePath,
		"auth_enabled", cfg.Server.Auth.Enabled,
	)
}

// buildScheduleStatus checks and returns schedule reconciliation status
func buildScheduleStatus(ctx context.Context, server *Server) (bool, string, gin.H) {
	ready := true
	healthStatus := "healthy"
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
			logger.FromContext(ctx).
				Warn("Readiness probe check failed due to reconciliation error", "error", lastError)
			scheduleStatus["status"] = "retrying"
			scheduleStatus["last_error"] = "reconciliation failed"
			scheduleStatus["last_error_message"] = lastError.Error()
			ready = false
			healthStatus = "not_ready"
		default:
			scheduleStatus["status"] = "initializing"
			ready = false
			healthStatus = "not_ready"
		}
	}

	return ready, healthStatus, scheduleStatus
}

// buildMemoryHealth retrieves and evaluates memory health status
func buildMemoryHealth(ctx context.Context, ready *bool, healthStatus *string) gin.H {
	var memoryHealth gin.H
	if globalHealthService := memory.GetGlobalHealthService(); globalHealthService != nil {
		memoryHealth = memory.GetMemoryHealthForMainEndpoint(ctx, globalHealthService)

		if memoryHealthy, exists := memoryHealth["healthy"].(bool); exists && !memoryHealthy {
			*ready = false
			*healthStatus = "degraded"
		}
	}
	return memoryHealth
}

// buildHealthResponse constructs the complete health check response
func buildHealthResponse(healthStatus, version string, ready bool, scheduleStatus, memoryHealth gin.H) gin.H {
	response := gin.H{
		"status":    healthStatus,
		"version":   version,
		"ready":     ready,
		"schedules": scheduleStatus,
	}

	if memoryHealth != nil {
		response["memory"] = memoryHealth
	}

	return response
}

// determineHealthStatusCode returns appropriate HTTP status code based on readiness
func determineHealthStatusCode(ready bool) int {
	if !ready {
		return http.StatusServiceUnavailable
	}
	return http.StatusOK
}
