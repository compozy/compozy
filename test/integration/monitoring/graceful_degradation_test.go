package monitoring_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/infra/monitoring"
)

func TestMonitoringGracefulDegradation(t *testing.T) {
	t.Run("Should handle nil monitoring service gracefully", func(t *testing.T) {
		// Create a disabled monitoring service
		config := &monitoring.Config{
			Enabled: false,
			Path:    "/metrics",
		}
		degradedService, err := monitoring.NewMonitoringService(t.Context(), config)
		require.NoError(t, err)
		// Create router with degraded middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(degradedService.GinMiddleware(t.Context()))
		// Add test route
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		// Create test server
		server := httptest.NewServer(router)
		defer server.Close()
		// Make request - should work even with degraded monitoring
		req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/test", http.NoBody)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	t.Run("Should return 503 for metrics endpoint when monitoring is degraded", func(t *testing.T) {
		// Create a disabled monitoring service
		config := &monitoring.Config{
			Enabled: false,
			Path:    "/metrics",
		}
		degradedService, err := monitoring.NewMonitoringService(t.Context(), config)
		require.NoError(t, err)
		// Create metrics endpoint with degraded handler
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/metrics", gin.WrapH(degradedService.ExporterHandler()))
		// Create test server
		server := httptest.NewServer(router)
		defer server.Close()
		// Request metrics endpoint
		req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/metrics", http.NoBody)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Should return 503 Service Unavailable
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})
	t.Run("Should handle disabled monitoring configuration", func(t *testing.T) {
		// Create monitoring service with disabled config
		config := &monitoring.Config{
			Enabled: false,
			Path:    "/metrics",
		}
		monitoringService, err := monitoring.NewMonitoringService(t.Context(), config)
		require.NoError(t, err)
		// Create router with monitoring middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(monitoringService.GinMiddleware(t.Context()))
		// Add test route
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		// Create test server
		server := httptest.NewServer(router)
		defer server.Close()
		// Make request - should work even with disabled monitoring
		req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/test", http.NoBody)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		// Metrics endpoint should also work but return no metrics
		metricsRouter := gin.New()
		metricsRouter.GET("/metrics", gin.WrapH(monitoringService.ExporterHandler()))
		metricsServer := httptest.NewServer(metricsRouter)
		defer metricsServer.Close()
		req, err = http.NewRequestWithContext(context.Background(), "GET", metricsServer.URL+"/metrics", http.NoBody)
		require.NoError(t, err)
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// With disabled monitoring, the handler should indicate the service is unavailable
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})
	t.Run("Should use fallback service for invalid configuration", func(t *testing.T) {
		// Create invalid config
		invalidConfig := &monitoring.Config{
			Enabled: true,
			Path:    "invalid-path", // Missing leading slash
		}
		// NewMonitoringServiceWithFallback should return degraded service
		monitoringService := monitoring.NewMonitoringServiceWithFallback(t.Context(), invalidConfig)
		assert.NotNil(t, monitoringService)
		// Verify it's using degraded service by checking metrics endpoint
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/metrics", gin.WrapH(monitoringService.ExporterHandler()))
		server := httptest.NewServer(router)
		defer server.Close()
		req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL+"/metrics", http.NoBody)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		// Should return 503 for degraded service
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})
	t.Run("Should handle temporal interceptor with degraded monitoring", func(t *testing.T) {
		// Create disabled monitoring service
		config := &monitoring.Config{
			Enabled: false,
			Path:    "/metrics",
		}
		degradedService, err := monitoring.NewMonitoringService(t.Context(), config)
		require.NoError(t, err)
		// Get temporal interceptor - should not panic
		interceptor := degradedService.TemporalInterceptor(t.Context())
		assert.NotNil(t, interceptor)
		// The interceptor should be a no-op but functional
		// We can't easily test the actual behavior without a full Temporal setup,
		// but we verify it doesn't panic when created
	})
	t.Run("Should gracefully shutdown degraded monitoring service", func(t *testing.T) {
		// Create disabled monitoring service
		config := &monitoring.Config{
			Enabled: false,
			Path:    "/metrics",
		}
		degradedService, err := monitoring.NewMonitoringService(t.Context(), config)
		require.NoError(t, err)
		// Shutdown should not panic or error
		ctx := context.Background()
		shutdownErr := degradedService.Shutdown(ctx)
		assert.NoError(t, shutdownErr)
	})
}
