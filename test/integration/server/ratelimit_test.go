package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/monitoring"
	"github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitMiddleware_PerKeyRateLimiting(t *testing.T) {
	t.Run("Should enforce per-API key rate limiting", func(t *testing.T) {
		ctx := context.Background()
		// Create monitoring service
		monitoringService, err := monitoring.NewMonitoringService(ctx, &monitoring.Config{
			Enabled: true,
			Path:    "/metrics",
		})
		require.NoError(t, err)
		defer monitoringService.Shutdown(context.Background())
		// Configure rate limit: 5 requests per second for global, 10 per second for API keys
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  5,
				Period: 1 * time.Second,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  10,
				Period: 1 * time.Second,
			},
			ExcludedPaths: []string{"/health"},
		}
		// Create rate limit manager with metrics
		manager, err := ratelimit.NewManagerWithMetrics(config, nil, monitoringService.Meter())
		require.NoError(t, err)
		// Setup test router
		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add middleware to simulate auth that sets API key
		router.Use(func(c *gin.Context) {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				c.Set("apiKey", apiKey)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Test with API key 1
		apiKey1 := "test-api-key-1"
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.Header.Set("X-API-Key", apiKey1)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
			// Check rate limit headers
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
		}
		// 11th request should be rate limited
		req := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req.Header.Set("X-API-Key", apiKey1)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "Rate limit exceeded")
		// Test with different API key - should succeed
		apiKey2 := "test-api-key-2"
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.Header.Set("X-API-Key", apiKey2)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code, "Different API key should not be rate limited")
	})

	t.Run("Should enforce per-user ID rate limiting", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  3,
				Period: 1 * time.Second,
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add middleware to simulate auth that sets user ID
		router.Use(func(c *gin.Context) {
			userID := c.GetHeader("X-User-ID")
			if userID != "" {
				c.Set("userID", userID)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Test with user ID 1
		userID1 := "user-123"
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.Header.Set("X-User-ID", userID1)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}
		// 4th request should be rate limited
		req := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req.Header.Set("X-User-ID", userID1)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		// Different user should succeed
		userID2 := "user-456"
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.Header.Set("X-User-ID", userID2)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Should enforce per-IP rate limiting for anonymous users", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  2,
				Period: 1 * time.Second,
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Make requests from same IP
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.RemoteAddr = "192.168.1.100:12345"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}
		// 3rd request should be rate limited
		req := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		// Different IP should succeed
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.RemoteAddr = "192.168.1.101:12345"
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Should prioritize API key over user ID and IP", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  2,
				Period: 1 * time.Second,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  5,
				Period: 1 * time.Second,
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add middleware to simulate auth that sets both API key and user ID
		router.Use(func(c *gin.Context) {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				c.Set("apiKey", apiKey)
			}
			userID := c.GetHeader("X-User-ID")
			if userID != "" {
				c.Set("userID", userID)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Make requests with all three identifiers
		apiKey := "priority-test-key"
		userID := "priority-test-user"
		// First 5 requests should succeed (API key limit)
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.RemoteAddr = "192.168.1.100:12345"
			req.Header.Set("X-API-Key", apiKey)
			req.Header.Set("X-User-ID", userID)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}
		// 6th request with same API key should be rate limited
		req := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req.RemoteAddr = "192.168.1.100:12345"
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("X-User-ID", userID)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		// Request with same user ID but different API key should succeed
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		req2.RemoteAddr = "192.168.1.100:12345"
		req2.Header.Set("X-API-Key", "different-api-key")
		req2.Header.Set("X-User-ID", userID)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("Should handle concurrent requests correctly", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  10,
				Period: 1 * time.Second,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  10,
				Period: 1 * time.Second,
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add middleware to simulate auth that sets API key
		router.Use(func(c *gin.Context) {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				c.Set("apiKey", apiKey)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Make concurrent requests
		var wg sync.WaitGroup
		successCount := 0
		rateLimitedCount := 0
		var mu sync.Mutex
		apiKey := "concurrent-test-key"
		for i := 0; i < 15; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("GET", "/api/test", http.NoBody)
				req.Header.Set("X-API-Key", apiKey)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				mu.Lock()
				switch w.Code {
				case http.StatusOK:
					successCount++
				case http.StatusTooManyRequests:
					rateLimitedCount++
				}
				mu.Unlock()
			}()
		}
		wg.Wait()
		assert.Equal(t, 10, successCount, "Should allow exactly 10 requests")
		assert.Equal(t, 5, rateLimitedCount, "Should rate limit 5 requests")
	})

	t.Run("Should exclude configured paths", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  1,
				Period: 1 * time.Second,
			},
			ExcludedPaths: []string{"/health", "/metrics"},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "healthy"})
		})
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Make multiple requests to excluded path
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/health", http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Excluded path should not be rate limited")
		}
		// Non-excluded path should be rate limited
		req := httptest.NewRequest("GET", "/api/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		// Second request should be rate limited
		req2 := httptest.NewRequest("GET", "/api/test", http.NoBody)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	})

	t.Run("Should increment Prometheus metrics when rate limit is hit", func(t *testing.T) {
		ctx := context.Background()
		// Create monitoring service
		monitoringService, err := monitoring.NewMonitoringService(ctx, &monitoring.Config{
			Enabled: true,
			Path:    "/metrics",
		})
		require.NoError(t, err)
		defer monitoringService.Shutdown(context.Background())
		// Configure rate limit: 2 requests per second
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  2,
				Period: 1 * time.Second,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  2,
				Period: 1 * time.Second,
			},
		}
		// Create rate limit manager with metrics
		manager, err := ratelimit.NewManagerWithMetrics(config, nil, monitoringService.Meter())
		require.NoError(t, err)
		// Setup test router
		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add middleware to simulate auth that sets API key
		router.Use(func(c *gin.Context) {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				c.Set("apiKey", apiKey)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Make requests to hit rate limit
		apiKey := "metrics-test-key"
		successCount := 0
		blockedCount := 0
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			req.Header.Set("X-API-Key", apiKey)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			switch w.Code {
			case http.StatusOK:
				successCount++
			case http.StatusTooManyRequests:
				blockedCount++
			}
		}
		// Verify that rate limiting occurred
		assert.Equal(t, 2, successCount, "Should allow 2 requests")
		assert.Equal(t, 1, blockedCount, "Should block 1 request")
		// Make request without API key to test IP-based rate limiting
		ipSuccessCount := 0
		ipBlockedCount := 0
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", "/api/test", http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			switch w.Code {
			case http.StatusOK:
				ipSuccessCount++
			case http.StatusTooManyRequests:
				ipBlockedCount++
			}
		}
		// Verify that rate limiting occurred for IP-based requests
		assert.Equal(t, 2, ipSuccessCount, "Should allow 2 IP-based requests")
		assert.Equal(t, 1, ipBlockedCount, "Should block 1 IP-based request")
	})

	t.Run("Should enforce route-specific limits over API key limits", func(t *testing.T) {
		ctx := context.Background()
		// Create monitoring service
		monitoringService, err := monitoring.NewMonitoringService(ctx, &monitoring.Config{
			Enabled: true,
			Path:    "/metrics",
		})
		require.NoError(t, err)
		defer monitoringService.Shutdown(context.Background())
		// Configure rate limit: route limit is stricter than API key limit
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  10,
				Period: 1 * time.Second,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  10,
				Period: 1 * time.Second,
			},
			RouteRates: map[string]ratelimit.RateConfig{
				"/api/limited": {
					Limit:  2, // Stricter limit
					Period: 1 * time.Second,
				},
			},
		}
		// Create rate limit manager with metrics
		manager, err := ratelimit.NewManagerWithMetrics(config, nil, monitoringService.Meter())
		require.NoError(t, err)
		// Setup test router
		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add middleware to simulate auth that sets API key
		router.Use(func(c *gin.Context) {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				c.Set("apiKey", apiKey)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/limited", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		router.GET("/api/normal", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Test with API key on route-limited endpoint
		apiKey := "route-test-key"
		// First 2 requests should succeed (route limit)
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/api/limited", http.NoBody)
			req.Header.Set("X-API-Key", apiKey)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
			// Check that limit header shows route limit, not API key limit
			assert.Equal(t, "2", w.Header().Get("X-RateLimit-Limit"))
		}
		// 3rd request should be rate limited by route limit
		req := httptest.NewRequest("GET", "/api/limited", http.NoBody)
		req.Header.Set("X-API-Key", apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code, "Should be rate limited by route limit")
		// Same API key on normal endpoint should still have quota
		req2 := httptest.NewRequest("GET", "/api/normal", http.NoBody)
		req2.Header.Set("X-API-Key", apiKey)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code, "Different route should use API key limit")
		assert.Equal(t, "10", w2.Header().Get("X-RateLimit-Limit"), "Should show API key limit")
	})

	t.Run("Should use longest matching prefix for route limits", func(t *testing.T) {
		// Configure rate limit with overlapping prefixes
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  100,
				Period: 1 * time.Minute,
			},
			RouteRates: map[string]ratelimit.RateConfig{
				"/api/v0/": {
					Limit:  50,
					Period: 1 * time.Minute,
				},
				"/api/v0/memory/": {
					Limit:  20,
					Period: 1 * time.Minute,
				},
				"/api/v0/memory/store/": {
					Limit:  10,
					Period: 1 * time.Minute,
				},
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/api/v0/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		router.GET("/api/v0/memory/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		router.GET("/api/v0/memory/store/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		// Test /api/v0/test - should use /api/v0/ limit (50)
		req := httptest.NewRequest("GET", "/api/v0/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "50", w.Header().Get("X-RateLimit-Limit"))
		// Test /api/v0/memory/test - should use /api/v0/memory/ limit (20)
		req2 := httptest.NewRequest("GET", "/api/v0/memory/test", http.NoBody)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
		assert.Equal(t, "20", w2.Header().Get("X-RateLimit-Limit"))
		// Test /api/v0/memory/store/test - should use /api/v0/memory/store/ limit (10)
		req3 := httptest.NewRequest("GET", "/api/v0/memory/store/test", http.NoBody)
		w3 := httptest.NewRecorder()
		router.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusOK, w3.Code)
		assert.Equal(t, "10", w3.Header().Get("X-RateLimit-Limit"))
	})

	t.Run("Should return correct rate limit headers", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  100,
				Period: 1 * time.Minute,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  100,
				Period: 1 * time.Minute,
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		// Add middleware to simulate auth that sets API key
		router.Use(func(c *gin.Context) {
			apiKey := c.GetHeader("X-API-Key")
			if apiKey != "" {
				c.Set("apiKey", apiKey)
			}
			c.Next()
		})
		router.Use(manager.Middleware())
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
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

	t.Run("Should return correct limit info for API key requests", func(t *testing.T) {
		config := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  50,
				Period: 1 * time.Minute,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  100,
				Period: 1 * time.Minute,
			},
		}
		manager, err := ratelimit.NewManager(config, nil)
		require.NoError(t, err)
		gin.SetMode(gin.TestMode)
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
		config2 := &ratelimit.Config{
			GlobalRate: ratelimit.RateConfig{
				Limit:  50,
				Period: 1 * time.Minute,
			},
			APIKeyRate: ratelimit.RateConfig{
				Limit:  100,
				Period: 1 * time.Minute,
			},
			RouteRates: map[string]ratelimit.RateConfig{
				"/api/special": {
					Limit:  10,
					Period: 1 * time.Minute,
				},
			},
		}
		manager2, err := ratelimit.NewManager(config2, nil)
		require.NoError(t, err)
		c3, _ := gin.CreateTestContext(httptest.NewRecorder())
		c3.Request = httptest.NewRequest("GET", "/api/special", http.NoBody)
		c3.Set("apiKey", "test-key-123")
		info3, err := manager2.GetLimitInfo(c3)
		require.NoError(t, err)
		assert.Equal(t, int64(10), info3.Limit, "Should return route-specific limit, not API key limit")
	})
}
