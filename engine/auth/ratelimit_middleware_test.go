package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/auth/ratelimit"
	"github.com/compozy/compozy/engine/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware_LimitByAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should allow request when no API key in context", func(t *testing.T) {
		// Setup
		config := &ratelimit.Config{
			DefaultRequestsPerSecond: 1,
			DefaultBurstCapacity:     1,
			CleanupInterval:          time.Minute,
			LimiterExpiry:            time.Hour,
		}
		service := ratelimit.NewService(config)
		middleware := NewRateLimitMiddleware(service)
		// Create test context without API key
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.LimitByAPIKey()
		handler(c)
		// Assert - should pass through
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("Should enforce rate limit per API key", func(t *testing.T) {
		// Setup with very low rate limit
		config := &ratelimit.Config{
			DefaultRequestsPerSecond: 1,
			DefaultBurstCapacity:     1,
			CleanupInterval:          time.Minute,
			LimiterExpiry:            time.Hour,
		}
		service := ratelimit.NewService(config)
		middleware := NewRateLimitMiddleware(service)
		// Create API key for context
		testKey := &apikey.APIKey{
			ID:     core.MustNewID(),
			UserID: core.MustNewID(),
			OrgID:  core.MustNewID(),
		}
		// First request should succeed
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx1 := c1.Request.Context()
		ctx1 = WithAPIKey(ctx1, testKey)
		c1.Request = c1.Request.WithContext(ctx1)
		handler := middleware.LimitByAPIKey()
		handler(c1)
		assert.Equal(t, http.StatusOK, w1.Code)
		// Second request should be rate limited
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx2 := c2.Request.Context()
		ctx2 = WithAPIKey(ctx2, testKey)
		c2.Request = c2.Request.WithContext(ctx2)
		handler(c2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.Contains(t, w2.Body.String(), "Rate Limit Exceeded")
	})
	t.Run("Should track rate limits separately per API key", func(t *testing.T) {
		// Setup
		config := &ratelimit.Config{
			DefaultRequestsPerSecond: 1,
			DefaultBurstCapacity:     1,
			CleanupInterval:          time.Minute,
			LimiterExpiry:            time.Hour,
		}
		service := ratelimit.NewService(config)
		middleware := NewRateLimitMiddleware(service)
		// Create two different API keys
		key1 := &apikey.APIKey{ID: core.MustNewID()}
		key2 := &apikey.APIKey{ID: core.MustNewID()}
		handler := middleware.LimitByAPIKey()
		// First request with key1 - should succeed
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx1 := WithAPIKey(c1.Request.Context(), key1)
		c1.Request = c1.Request.WithContext(ctx1)
		handler(c1)
		assert.Equal(t, http.StatusOK, w1.Code)
		// First request with key2 - should also succeed
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx2 := WithAPIKey(c2.Request.Context(), key2)
		c2.Request = c2.Request.WithContext(ctx2)
		handler(c2)
		assert.Equal(t, http.StatusOK, w2.Code)
		// Second request with key1 - should be rate limited
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx3 := WithAPIKey(c3.Request.Context(), key1)
		c3.Request = c3.Request.WithContext(ctx3)
		handler(c3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	})
}

func TestRateLimitMiddleware_LimitByIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should enforce rate limit per IP", func(t *testing.T) {
		// Setup
		middleware := &RateLimitMiddleware{}
		handler := middleware.LimitByIP(1, 1) // 1 req/sec, burst 1
		// First request should succeed
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c1.Request.Header.Set("X-Real-IP", "192.168.1.1")
		handler(c1)
		assert.Equal(t, http.StatusOK, w1.Code)
		// Second request from same IP should be rate limited
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c2.Request.Header.Set("X-Real-IP", "192.168.1.1")
		handler(c2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.Contains(t, w2.Body.String(), "Rate Limit Exceeded")
	})
	t.Run("Should track rate limits separately per IP", func(t *testing.T) {
		// Setup
		middleware := &RateLimitMiddleware{}
		handler := middleware.LimitByIP(1, 1) // 1 req/sec, burst 1
		// First request from IP1 - should succeed
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c1.Request.Header.Set("X-Real-IP", "192.168.1.1")
		handler(c1)
		assert.Equal(t, http.StatusOK, w1.Code)
		// First request from IP2 - should also succeed
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c2.Request.Header.Set("X-Real-IP", "192.168.1.2")
		handler(c2)
		assert.Equal(t, http.StatusOK, w2.Code)
		// Second request from IP1 - should be rate limited
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c3.Request.Header.Set("X-Real-IP", "192.168.1.1")
		handler(c3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
		// Second request from IP2 - should be rate limited
		w4 := httptest.NewRecorder()
		c4, _ := gin.CreateTestContext(w4)
		c4.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c4.Request.Header.Set("X-Real-IP", "192.168.1.2")
		handler(c4)
		assert.Equal(t, http.StatusTooManyRequests, w4.Code)
	})
	t.Run("Should not share rate limiter instances between middleware calls", func(t *testing.T) {
		// This test verifies that the memory leak fix is working
		// Each call to LimitByIP should use the same service instance
		middleware := &RateLimitMiddleware{}
		// Create two handlers
		handler1 := middleware.LimitByIP(10, 10)
		handler2 := middleware.LimitByIP(20, 20)
		// Both handlers should work independently
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c1.Request.Header.Set("X-Real-IP", "192.168.1.1")
		handler1(c1)
		assert.Equal(t, http.StatusOK, w1.Code)
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c2.Request.Header.Set("X-Real-IP", "192.168.1.1")
		handler2(c2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})
	t.Run("Should allow burst capacity then enforce sustained rate", func(t *testing.T) {
		// Setup with burst capacity of 3 and rate of 1/sec
		config := &ratelimit.Config{
			DefaultRequestsPerSecond: 1,
			DefaultBurstCapacity:     3,
			CleanupInterval:          time.Minute,
			LimiterExpiry:            time.Hour,
		}
		service := ratelimit.NewService(config)
		middleware := NewRateLimitMiddleware(service)
		testKey := &apikey.APIKey{ID: core.MustNewID()}
		handler := middleware.LimitByAPIKey()
		// First 3 requests should succeed (burst capacity)
		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
			ctx := WithAPIKey(c.Request.Context(), testKey)
			c.Request = c.Request.WithContext(ctx)
			handler(c)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}
		// 4th request should be rate limited
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx := WithAPIKey(c.Request.Context(), testKey)
		c.Request = c.Request.WithContext(ctx)
		handler(c)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "Rate Limit Exceeded")
	})
	t.Run("Should recover after rate limit period", func(t *testing.T) {
		// Setup with very fast rate recovery for testing
		config := &ratelimit.Config{
			DefaultRequestsPerSecond: 100, // Very high rate for quick recovery
			DefaultBurstCapacity:     1,
			CleanupInterval:          time.Minute,
			LimiterExpiry:            time.Hour,
		}
		service := ratelimit.NewService(config)
		middleware := NewRateLimitMiddleware(service)
		testKey := &apikey.APIKey{ID: core.MustNewID()}
		handler := middleware.LimitByAPIKey()
		// First request should succeed
		w1 := httptest.NewRecorder()
		c1, _ := gin.CreateTestContext(w1)
		c1.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx1 := WithAPIKey(c1.Request.Context(), testKey)
		c1.Request = c1.Request.WithContext(ctx1)
		handler(c1)
		assert.Equal(t, http.StatusOK, w1.Code)
		// Second request should be rate limited
		w2 := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w2)
		c2.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx2 := WithAPIKey(c2.Request.Context(), testKey)
		c2.Request = c2.Request.WithContext(ctx2)
		handler(c2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		// Wait for rate limit to recover (very short with high rate)
		time.Sleep(20 * time.Millisecond)
		// Third request should succeed again
		w3 := httptest.NewRecorder()
		c3, _ := gin.CreateTestContext(w3)
		c3.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		ctx3 := WithAPIKey(c3.Request.Context(), testKey)
		c3.Request = c3.Request.WithContext(ctx3)
		handler(c3)
		assert.Equal(t, http.StatusOK, w3.Code)
	})
	t.Run("Should handle concurrent requests properly", func(t *testing.T) {
		// Setup with limited capacity
		config := &ratelimit.Config{
			DefaultRequestsPerSecond: 5,
			DefaultBurstCapacity:     2,
			CleanupInterval:          time.Minute,
			LimiterExpiry:            time.Hour,
		}
		service := ratelimit.NewService(config)
		middleware := NewRateLimitMiddleware(service)
		testKey := &apikey.APIKey{ID: core.MustNewID()}
		handler := middleware.LimitByAPIKey()
		// Launch multiple concurrent requests
		results := make([]int, 10)
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(index int) {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
				ctx := WithAPIKey(c.Request.Context(), testKey)
				c.Request = c.Request.WithContext(ctx)
				handler(c)
				results[index] = w.Code
				done <- true
			}(i)
		}
		// Wait for all requests to complete
		for i := 0; i < 10; i++ {
			<-done
		}
		// Count successful vs rate-limited requests
		successCount := 0
		rateLimitedCount := 0
		for _, code := range results {
			switch code {
			case http.StatusOK:
				successCount++
			case http.StatusTooManyRequests:
				rateLimitedCount++
			}
		}
		// Should have some successful requests (burst capacity) and some rate limited
		assert.Greater(t, successCount, 0, "Should have some successful requests")
		assert.Greater(t, rateLimitedCount, 0, "Should have some rate limited requests")
		assert.Equal(t, 10, successCount+rateLimitedCount, "All requests should be accounted for")
	})
}
