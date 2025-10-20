package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	helpers "github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test fixture for rate limit tests
type rateLimitTestFixture struct {
	manager *ratelimit.Manager
	router  *gin.Engine
}

// setupRateLimitFixture creates a reusable test fixture with minimal config
func setupRateLimitFixture(t *testing.T, globalLimit, apiKeyLimit int64) *rateLimitTestFixture {
	config := &ratelimit.Config{
		GlobalRate: ratelimit.RateConfig{Limit: globalLimit, Period: 1 * time.Second},
		APIKeyRate: ratelimit.RateConfig{Limit: apiKeyLimit, Period: 1 * time.Second},
	}
	manager, err := ratelimit.NewManager(config, nil)
	require.NoError(t, err)
	ginmode.EnsureGinTestMode()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			c.Set("apiKey", apiKey)
		}
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			c.Set("userID", userID)
		}
		c.Next()
	})
	router.Use(manager.Middleware())
	router.GET("/api/test", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	return &rateLimitTestFixture{manager, router}
}

func TestRateLimitMiddleware_PerKeyRateLimiting(t *testing.T) {
	t.Run("Should enforce per-API key rate limiting", func(t *testing.T) {
		fixture := setupRateLimitFixture(t, 5, 3) // Lower limits for faster testing
		apiKey1 := "test-api-key-1"
		// Test within limit and over limit
		for i := range 4 {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.Header.Set("X-API-Key", apiKey1)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if i < 3 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
				assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
				assert.True(t, helpers.Contains(w.Body.String(), "rate limit exceeded"))
			}
		}
		// Test different API key has separate limit
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.Header.Set("X-API-Key", "test-api-key-2")
		w2 := httptest.NewRecorder()
		fixture.router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code, "Different API key should have separate limit")
	})

	t.Run("Should enforce per-user ID rate limiting", func(t *testing.T) {
		fixture := setupRateLimitFixture(t, 2, 10) // Global limit lower than API key
		userID1 := "user-123"
		// Test within limit and over limit
		for i := range 3 {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.Header.Set("X-User-ID", userID1)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if i < 2 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
			}
		}
		// Different user should succeed
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.Header.Set("X-User-ID", "user-456")
		w2 := httptest.NewRecorder()
		fixture.router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Should enforce per-IP rate limiting for anonymous users", func(t *testing.T) {
		fixture := setupRateLimitFixture(t, 2, 10)
		// Test same IP rate limiting
		for i := range 3 {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.RemoteAddr = "192.168.1.100:12345"
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if i < 2 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
			}
		}
		// Different IP should succeed
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.RemoteAddr = "192.168.1.101:12345"
		w2 := httptest.NewRecorder()
		fixture.router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Should prioritize API key over user ID and IP", func(t *testing.T) {
		fixture := setupRateLimitFixture(t, 2, 5)
		apiKey := "priority-test-key"
		userID := "priority-test-user"
		// First 5 requests should succeed (API key limit)
		for i := range 6 {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.RemoteAddr = "192.168.1.100:12345"
			req.Header.Set("X-API-Key", apiKey)
			req.Header.Set("X-User-ID", userID)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if i < 5 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
			}
		}
		// Request with different API key should succeed
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.RemoteAddr = "192.168.1.100:12345"
		req2.Header.Set("X-API-Key", "different-api-key")
		req2.Header.Set("X-User-ID", userID)
		w2 := httptest.NewRecorder()
		fixture.router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Should handle concurrent requests correctly", func(t *testing.T) {
		fixture := setupRateLimitFixture(t, 10, 10)
		var wg sync.WaitGroup
		successCount := 0
		rateLimitedCount := 0
		var mu sync.Mutex
		apiKey := "concurrent-test-key"
		for range 15 {
			wg.Go(func() {
				req := httptest.NewRequest("GET", "/api/test", http.NoBody)
				req.Header.Set("X-API-Key", apiKey)
				w := httptest.NewRecorder()
				fixture.router.ServeHTTP(w, req)
				mu.Lock()
				switch w.Code {
				case http.StatusOK:
					successCount++
				case http.StatusTooManyRequests:
					rateLimitedCount++
				}
				mu.Unlock()
			})
		}
		wg.Wait()
		assert.Equal(t, 10, successCount, "Should allow exactly 10 requests")
		assert.Equal(t, 5, rateLimitedCount, "Should rate limit 5 requests")
	})

	t.Run("Should exclude configured paths and handle route limits", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate:    ratelimit.RateConfig{Limit: 1, Period: 1 * time.Second},
			ExcludedPaths: []string{"/health", "/metrics"},
			RouteRates: map[string]ratelimit.RateConfig{
				"/api/limited": {Limit: 2, Period: 1 * time.Second},
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		ginmode.EnsureGinTestMode()
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "healthy"}) })
		router.GET("/api/test", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
		router.GET("/api/limited", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
		// Test excluded path - multiple requests should succeed
		for range 3 {
			req := httptest.NewRequest("GET", "/health", http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Excluded path should not be rate limited")
		}
		// Test route-specific limit
		for i := range 3 {
			req := httptest.NewRequest("GET", "/api/limited", http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if i < 2 {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "2", w.Header().Get("X-RateLimit-Limit"))
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
			}
		}
	})

	t.Run("Should handle metrics and headers correctly", func(t *testing.T) {
		ctx := t.Context()
		monitoringService, err := monitoring.NewMonitoringService(ctx, &monitoring.Config{
			Enabled: true,
			Path:    "/metrics",
		})
		require.NoError(t, err)
		defer monitoringService.Shutdown(t.Context())
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{Limit: 100, Period: 1 * time.Minute},
			APIKeyRate: ratelimit.RateConfig{Limit: 100, Period: 1 * time.Minute},
		}
		manager, err := ratelimit.NewManagerWithMetrics(ctx, config, nil, monitoringService.Meter())
		require.NoError(t, err)
		ginmode.EnsureGinTestMode()
		router := gin.New()
		router.Use(func(c *gin.Context) {
			if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
				c.Set("apiKey", apiKey)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
		apiKey := "header-test-key"
		req := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req.Header.Set("X-API-Key", apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		// Check headers
		limitHeader := w.Header().Get("X-RateLimit-Limit")
		remainingHeader := w.Header().Get("X-RateLimit-Remaining")
		resetHeader := w.Header().Get("X-RateLimit-Reset")
		assert.Equal(t, "100", limitHeader)
		remaining, err := strconv.Atoi(remainingHeader)
		assert.NoError(t, err)
		assert.Equal(t, 99, remaining, "Should have 99 requests remaining")
		resetTime, err := strconv.ParseInt(resetHeader, 10, 64)
		assert.NoError(t, err)
		assert.Greater(t, resetTime, time.Now().Unix(), "Reset time should be in the future")
	})

	t.Run("Should return correct limit info for different contexts", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{Limit: 50, Period: 1 * time.Minute},
			APIKeyRate: ratelimit.RateConfig{Limit: 100, Period: 1 * time.Minute},
			RouteRates: map[string]ratelimit.RateConfig{
				"/api/special": {Limit: 10, Period: 1 * time.Minute},
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		ginmode.EnsureGinTestMode()
		// Test with API key - should get API key limiter info
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/api/test", http.NoBody)
		c.Set("apiKey", "test-key-123")
		info, err := manager.GetLimitInfo(c)
		require.NoError(t, err)
		assert.Equal(t, int64(100), info.Limit, "Should return API key limit, not global limit")
		// Test without API key - should get global limiter info
		c2, _ := gin.CreateTestContext(httptest.NewRecorder())
		c2.Request = httptest.NewRequest("GET", "/api/test", http.NoBody)
		info2, err := manager.GetLimitInfo(c2)
		require.NoError(t, err)
		assert.Equal(t, int64(50), info2.Limit, "Should return global limit")
		// Test with route-specific limit
		c3, _ := gin.CreateTestContext(httptest.NewRecorder())
		c3.Request = httptest.NewRequest("GET", "/api/special", http.NoBody)
		c3.Set("apiKey", "test-key-123")
		info3, err := manager.GetLimitInfo(c3)
		require.NoError(t, err)
		assert.Equal(t, int64(10), info3.Limit, "Should return route-specific limit")
	})
}
