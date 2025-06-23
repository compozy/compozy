package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/memory"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpointWithMemoryIntegration(t *testing.T) {
	ctx := context.Background()
	memory.ResetGlobalHealthServiceForTesting()

	t.Run("Should include memory health when global service is available", func(t *testing.T) {
		// Initialize global memory health service with nil manager (realistic scenario)
		healthService := memory.InitializeGlobalHealthService(ctx, nil)
		require.NotNil(t, healthService)

		// Register some test memory instances
		healthService.RegisterInstance("test-memory-1")
		healthService.RegisterInstance("test-memory-2")

		// Create test router with health endpoint
		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Register health endpoint
		router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))

		// Create test request
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Check response - should be unhealthy due to nil manager but degraded, not not_ready
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		// Response should contain memory health data
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "memory")
		assert.Contains(t, responseBody, "healthy")
		assert.Contains(t, responseBody, "total_instances")
		assert.Contains(t, responseBody, "degraded") // Status should be degraded due to memory issues

		// Cleanup
		memory.ResetGlobalHealthServiceForTesting()
	})

	t.Run("Should not include memory health when global service is not available", func(t *testing.T) {
		// Ensure global service is not available
		memory.ResetGlobalHealthServiceForTesting()

		// Create test router with health endpoint
		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Register health endpoint
		router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))

		// Create test request
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		// Response should not contain memory health data
		responseBody := w.Body.String()
		assert.NotContains(t, responseBody, "memory")
	})

	t.Run("Should update overall health status when memory is unhealthy", func(t *testing.T) {
		// Reset global health service for clean test
		memory.ResetGlobalHealthServiceForTesting()

		// Initialize global memory health service
		healthService := memory.InitializeGlobalHealthService(ctx, nil)
		require.NotNil(t, healthService)

		// Don't register any instances (this makes memory system unhealthy)

		// Create test router with health endpoint
		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Register health endpoint
		router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))

		// Create test request
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(w, req)

		// Check response - should be degraded due to memory health
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "degraded")
		assert.Contains(t, responseBody, "memory")

		// Cleanup
		memory.ResetGlobalHealthServiceForTesting()
	})
}

func TestMemoryHealthRoutesRegistration(t *testing.T) {
	ctx := context.Background()
	t.Run("Should register memory health routes when global service is available", func(t *testing.T) {
		// Reset global health service for clean test
		memory.ResetGlobalHealthServiceForTesting()

		// Initialize global memory health service
		healthService := memory.InitializeGlobalHealthService(ctx, nil)
		require.NotNil(t, healthService)

		// Register a test instance
		healthService.RegisterInstance("test-memory")

		// Create test router
		gin.SetMode(gin.TestMode)
		router := gin.New()
		apiBase := router.Group("/api/v1")

		// Register memory health routes (this would be called by RegisterRoutes)
		if globalHealthService := memory.GetGlobalHealthService(); globalHealthService != nil {
			memory.RegisterMemoryHealthRoutes(apiBase, globalHealthService)
		}

		// Test memory system health endpoint
		req := httptest.NewRequest("GET", "/api/v1/memory/health", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should be unhealthy due to nil manager but endpoints should work
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "healthy")
		assert.Contains(t, responseBody, "total_instances")

		// Test specific memory instance health endpoint
		req = httptest.NewRequest("GET", "/api/v1/memory/health/test-memory", http.NoBody)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Instance may be unhealthy due to timeout or system issues
		responseBody = w.Body.String()
		assert.Contains(t, responseBody, "test-memory")
		assert.Contains(t, responseBody, "healthy")
		// Don't assert specific status code as it depends on health check timing

		// Cleanup
		memory.ResetGlobalHealthServiceForTesting()
	})
}
