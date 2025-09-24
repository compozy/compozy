package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/server"
	"github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
)

// BenchmarkHealthEndpoint measures health endpoint performance
func BenchmarkHealthEndpoint(b *testing.B) {
	ginmode.EnsureGinTestMode()
	router := gin.New()
	router.GET("/health", server.CreateHealthHandler(nil, "v1.0.0"))
	req := httptest.NewRequest("GET", "/health", http.NoBody)

	for b.Loop() {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkRateLimitMiddleware measures rate limit middleware performance
func BenchmarkRateLimitMiddleware(b *testing.B) {
	config := &ratelimit.Config{
		GlobalRate: ratelimit.RateConfig{Limit: 1000, Period: 1 * time.Second},
		APIKeyRate: ratelimit.RateConfig{Limit: 1000, Period: 1 * time.Second},
	}
	manager, err := ratelimit.NewManager(config, nil)
	if err != nil {
		b.Fatal(err)
	}
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
	req := httptest.NewRequest("GET", "/api/test", http.NoBody)
	req.Header.Set("X-API-Key", "bench-test-key")

	for b.Loop() {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkTestFixtureSetup measures the performance of our optimized test fixtures
func BenchmarkTestFixtureSetup(b *testing.B) {
	for b.Loop() {
		_ = setupRateLimitFixture(&testing.T{}, 100, 100)
	}
}
