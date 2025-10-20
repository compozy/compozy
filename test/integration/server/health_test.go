package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/memory"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// healthTestFixture provides reusable setup for health tests
type healthTestFixture struct {
	router *gin.Engine
	ctx    context.Context
}

// setupHealthTestFixture creates a reusable test fixture
func setupHealthTestFixture(t *testing.T) *healthTestFixture {
	t.Helper()
	ctx := t.Context()
	ginmode.EnsureGinTestMode()
	router := gin.New()
	memory.ResetGlobalHealthServiceForTesting()
	return &healthTestFixture{router, ctx}
}

func TestHealthEndpointWithMemoryIntegration(t *testing.T) {
	t.Run("Should include memory health when global service is available", func(t *testing.T) {
		fixture := setupHealthTestFixture(t)
		defer memory.ResetGlobalHealthServiceForTesting()
		healthService := memory.InitializeGlobalHealthService(fixture.ctx, nil)
		require.NotNil(t, healthService)
		healthService.RegisterInstance("test-memory-1")
		healthService.RegisterInstance("test-memory-2")
		fixture.router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "memory")
		assert.Contains(t, responseBody, "healthy")
		assert.Contains(t, responseBody, "total_instances")
		assert.Contains(t, responseBody, "degraded")
	})

	t.Run("Should not include memory health when global service is not available", func(t *testing.T) {
		fixture := setupHealthTestFixture(t)
		fixture.router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		responseBody := w.Body.String()
		assert.NotContains(t, responseBody, "memory")
		assert.Contains(t, responseBody, "healthy")
		assert.Contains(t, responseBody, "version")
	})

	t.Run("Should update overall health status when memory is unhealthy", func(t *testing.T) {
		fixture := setupHealthTestFixture(t)
		defer memory.ResetGlobalHealthServiceForTesting()
		healthService := memory.InitializeGlobalHealthService(fixture.ctx, nil)
		require.NotNil(t, healthService)
		// Don't register any instances (this makes memory system unhealthy)
		fixture.router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "degraded")
		assert.Contains(t, responseBody, "memory")
	})

	t.Run("Should handle nil server gracefully", func(t *testing.T) {
		fixture := setupHealthTestFixture(t)
		// Test with nil server (default case)
		fixture.router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "schedules")
		assert.Contains(t, responseBody, "ready")
		assert.Contains(t, responseBody, "healthy")
		// Should contain default reconciliation status
		assert.Contains(t, responseBody, `"reconciled":true`)
		assert.Contains(t, responseBody, `"status":"ready"`)
	})
}

func TestMemoryHealthRoutesRegistration(t *testing.T) {
	t.Run("Should register memory health routes when global service is available", func(t *testing.T) {
		fixture := setupHealthTestFixture(t)
		defer memory.ResetGlobalHealthServiceForTesting()
		healthService := memory.InitializeGlobalHealthService(fixture.ctx, nil)
		require.NotNil(t, healthService)
		healthService.RegisterInstance("test-memory")
		apiBase := fixture.router.Group("/api/v1")
		if globalHealthService := memory.GetGlobalHealthService(); globalHealthService != nil {
			memory.RegisterMemoryHealthRoutes(apiBase, globalHealthService)
		}
		// Test memory system health endpoint
		req := httptest.NewRequest("GET", "/api/v1/memory/health", http.NoBody)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "healthy")
		assert.Contains(t, responseBody, "total_instances")
		// Test specific memory instance health endpoint
		req = httptest.NewRequest("GET", "/api/v1/memory/health/test-memory", http.NoBody)
		w = httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		responseBody = w.Body.String()
		assert.Contains(t, responseBody, "test-memory")
		assert.Contains(t, responseBody, "healthy")
	})

	t.Run("Should handle health endpoint performance", func(t *testing.T) {
		fixture := setupHealthTestFixture(t)
		fixture.router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))
		// Test that health endpoint responds quickly
		start := time.Now()
		req := httptest.NewRequest("GET", "/health", http.NoBody)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		duration := time.Since(start)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Less(t, duration, 100*time.Millisecond, "Health endpoint should respond quickly")
		// Test response structure
		responseBody := w.Body.String()
		assert.Contains(t, responseBody, "status")
		assert.Contains(t, responseBody, "version")
		assert.Contains(t, responseBody, "ready")
	})
}
