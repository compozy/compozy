package monitoring_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/router"
	utils "github.com/compozy/compozy/test/helpers"
	"github.com/compozy/compozy/test/integration/worker/helpers"
)

// TestEnvironment provides a complete test environment for monitoring integration tests
type TestEnvironment struct {
	t                *testing.T
	temporalHelper   *helpers.TemporalHelper
	temporalSuite    *testsuite.WorkflowTestSuite
	httpServer       *httptest.Server
	monitoringServer *httptest.Server
	monitoring       *monitoring.Service
	router           *gin.Engine
	metricsURL       string
}

// SetupTestEnvironment creates a configured test environment with all services
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	// Initialize logger for tests
	utils.InitLogger(t)
	// Initialize monitoring service with test configuration
	config := &monitoring.Config{
		Enabled: true,
		Path:    "/metrics",
	}
	monitoringService, err := monitoring.NewMonitoringService(config)
	require.NoError(t, err)
	// Initialize Gin router with monitoring middleware
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(monitoringService.GinMiddleware())
	// Add test routes
	setupTestRoutes(r)
	// Create HTTP test server
	httpServer := httptest.NewServer(r)
	// Create separate metrics endpoint server using the same monitoring service
	metricsRouter := gin.New()
	metricsRouter.GET("/metrics", gin.WrapH(monitoringService.ExporterHandler()))
	monitoringServer := httptest.NewServer(metricsRouter)
	// Initialize Temporal test suite
	temporalSuite := &testsuite.WorkflowTestSuite{}
	temporalHelper := helpers.NewTemporalHelper(t, temporalSuite, "test-task-queue")
	env := &TestEnvironment{
		t:                t,
		temporalHelper:   temporalHelper,
		temporalSuite:    temporalSuite,
		httpServer:       httpServer,
		monitoringServer: monitoringServer,
		monitoring:       monitoringService,
		router:           r,
		metricsURL:       monitoringServer.URL + "/metrics",
	}
	return env
}

// Cleanup closes all test resources
func (env *TestEnvironment) Cleanup() {
	if env.httpServer != nil {
		env.httpServer.Close()
	}
	if env.monitoringServer != nil {
		env.monitoringServer.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = env.monitoring.Shutdown(ctx)
}

// GetHTTPClient returns an HTTP client configured for the test server
func (env *TestEnvironment) GetHTTPClient() *http.Client {
	return env.httpServer.Client()
}

// GetMetricsClient returns an HTTP client configured for the metrics server
func (env *TestEnvironment) GetMetricsClient() *http.Client {
	return env.monitoringServer.Client()
}

// MakeRequest makes a request to the test HTTP server
func (env *TestEnvironment) MakeRequest(method, path string) (*http.Response, error) {
	req, err := http.NewRequest(method, env.httpServer.URL+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	return env.GetHTTPClient().Do(req)
}

// GetMetrics fetches the current metrics from the metrics endpoint
func (env *TestEnvironment) GetMetrics() (string, error) {
	resp, err := env.GetMetricsClient().Get(env.metricsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metrics endpoint returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read metrics response body: %w", err)
	}
	return string(body), nil
}

// setupTestRoutes adds test routes to the router
func setupTestRoutes(r *gin.Engine) {
	// API routes for testing
	api := r.Group("/api/v1")
	{
		// Static routes
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, router.Response{
				Status:  http.StatusOK,
				Message: "OK",
			})
		})
		// Dynamic routes for cardinality testing
		api.GET("/users/:id", func(c *gin.Context) {
			c.JSON(http.StatusOK, router.Response{
				Status:  http.StatusOK,
				Message: "User found",
				Data:    map[string]string{"id": c.Param("id")},
			})
		})
		api.GET("/workflows/:workflow_id/executions/:exec_id", func(c *gin.Context) {
			c.JSON(http.StatusOK, router.Response{
				Status:  http.StatusOK,
				Message: "Execution found",
				Data: map[string]string{
					"workflow_id": c.Param("workflow_id"),
					"exec_id":     c.Param("exec_id"),
				},
			})
		})
		// Error routes for testing
		api.GET("/error", func(c *gin.Context) {
			c.JSON(http.StatusInternalServerError, router.Response{
				Status:  http.StatusInternalServerError,
				Message: "Internal server error",
			})
		})
		api.GET("/not-found", func(c *gin.Context) {
			c.JSON(http.StatusNotFound, router.Response{
				Status:  http.StatusNotFound,
				Message: "Resource not found",
			})
		})
	}
	// Non-API routes
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Welcome"})
	})
	// Handle 404 for unmatched routes
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, router.Response{
			Status:  http.StatusNotFound,
			Message: "Route not found",
		})
	})
}
