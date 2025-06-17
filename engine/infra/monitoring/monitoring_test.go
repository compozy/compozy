package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
)

func init() {
	logger.InitForTests()
}

func TestNewMonitoringService(t *testing.T) {
	t.Run("Should create service with default config when nil provided", func(t *testing.T) {
		service, err := NewMonitoringService(nil)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.NotNil(t, service.config)
		assert.False(t, service.config.Enabled)
		assert.Equal(t, "/metrics", service.config.Path)
		assert.False(t, service.IsInitialized())
	})
	t.Run("Should create service with provided config", func(t *testing.T) {
		cfg := &Config{
			Enabled: false,
			Path:    "/custom/metrics",
		}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, cfg, service.config)
		assert.False(t, service.IsInitialized())
	})
	t.Run("Should fail with invalid config", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "",
		}
		service, err := NewMonitoringService(cfg)
		assert.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "monitoring path cannot be empty")
	})
	t.Run("Should initialize with Prometheus exporter when enabled", func(t *testing.T) {
		cfg := &Config{
			Enabled: true,
			Path:    "/metrics",
		}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.True(t, service.IsInitialized())
		assert.NotNil(t, service.exporter)
		assert.NotNil(t, service.provider)
		assert.NotNil(t, service.meter)
		assert.Nil(t, service.InitializationError())
	})
	t.Run("Should use no-op meter when disabled", func(t *testing.T) {
		cfg := &Config{
			Enabled: false,
			Path:    "/metrics",
		}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.False(t, service.IsInitialized())
		assert.Nil(t, service.exporter)
		assert.Nil(t, service.provider)
		assert.NotNil(t, service.meter)
	})
}

func TestMonitoringService_Meter(t *testing.T) {
	t.Run("Should return meter instance", func(t *testing.T) {
		cfg := &Config{Enabled: true, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		meter := service.Meter()
		assert.NotNil(t, meter)
		assert.Implements(t, (*metric.Meter)(nil), meter)
	})
}

func TestMonitoringService_GinMiddleware(t *testing.T) {
	t.Run("Should return functional middleware when initialized", func(t *testing.T) {
		cfg := &Config{Enabled: true, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		middleware := service.GinMiddleware()
		assert.NotNil(t, middleware)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
	t.Run("Should return no-op middleware when not initialized", func(t *testing.T) {
		cfg := &Config{Enabled: false, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		middleware := service.GinMiddleware()
		assert.NotNil(t, middleware)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestMonitoringService_ExporterHandler(t *testing.T) {
	t.Run("Should return 503 when not initialized", func(t *testing.T) {
		cfg := &Config{Enabled: false, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		handler := service.ExporterHandler()
		req := httptest.NewRequest("GET", "/metrics", http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Contains(t, w.Body.String(), "Monitoring service not initialized")
	})
	t.Run("Should return metrics when initialized", func(t *testing.T) {
		cfg := &Config{Enabled: true, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		handler := service.ExporterHandler()
		req := httptest.NewRequest("GET", "/metrics", http.NoBody)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Logf("Response body: %s", w.Body.String())
		}
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	})
}

func TestMonitoringService_Shutdown(t *testing.T) {
	t.Run("Should shutdown gracefully when initialized", func(t *testing.T) {
		cfg := &Config{Enabled: true, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		err = service.Shutdown(context.Background())
		assert.NoError(t, err)
	})
	t.Run("Should handle shutdown when not initialized", func(t *testing.T) {
		cfg := &Config{Enabled: false, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		err = service.Shutdown(context.Background())
		assert.NoError(t, err)
	})
}

func TestMonitoringService_TemporalInterceptor(t *testing.T) {
	t.Run("Should return interceptor", func(t *testing.T) {
		cfg := &Config{Enabled: true, Path: "/metrics"}
		service, err := NewMonitoringService(cfg)
		require.NoError(t, err)
		interceptor := service.TemporalInterceptor()
		assert.NotNil(t, interceptor)
	})
}

func TestNewMonitoringServiceWithFallback(t *testing.T) {
	t.Run("Should return initialized service when config is valid", func(t *testing.T) {
		cfg := &Config{Enabled: true, Path: "/metrics"}
		service := NewMonitoringServiceWithFallback(cfg)
		assert.NotNil(t, service)
		assert.True(t, service.IsInitialized())
		assert.Nil(t, service.InitializationError())
	})
	t.Run("Should return degraded service when config is invalid", func(t *testing.T) {
		cfg := &Config{Enabled: true, Path: "invalid-path"}
		service := NewMonitoringServiceWithFallback(cfg)
		assert.NotNil(t, service)
		assert.False(t, service.IsInitialized())
		assert.NotNil(t, service.InitializationError())
		assert.NotNil(t, service.Meter())
	})
	t.Run("Should handle nil config gracefully", func(t *testing.T) {
		service := NewMonitoringServiceWithFallback(nil)
		assert.NotNil(t, service)
		assert.False(t, service.IsInitialized())
		assert.Nil(t, service.InitializationError())
	})
}
