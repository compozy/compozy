package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildRouterForTest(t *testing.T, cfg *Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m, err := NewManager(cfg, nil) // nil redis -> in-memory store
	require.NoError(t, err)
	r.Use(m.Middleware())
	r.GET("/t", func(c *gin.Context) { c.String(200, "ok") })
	return r
}

func doReq(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", http.NoBody)
	if ip != "" {
		req.Header.Set("X-Real-IP", ip)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestInMemoryGlobalRateLimit_BlocksSecondRequest(t *testing.T) {
	cfg := &Config{
		GlobalRate: RateConfig{Limit: 1, Period: time.Second},
		APIKeyRate: RateConfig{Limit: 100, Period: time.Minute},
		RouteRates: map[string]RateConfig{},
		Prefix:     "test:ratelimit:",
		MaxRetry:   1,
	}
	r := buildRouterForTest(t, cfg)

	// First request should pass
	res1 := doReq(r, "1.2.3.4")
	require.Equal(t, 200, res1.Code)
	// Second immediate request should be blocked (same IP key)
	res2 := doReq(r, "1.2.3.4")
	require.Equal(t, 429, res2.Code)
}

func TestInMemoryGlobalRateLimit_RefillAfterPeriod(t *testing.T) {
	cfg := &Config{
		GlobalRate: RateConfig{Limit: 1, Period: 100 * time.Millisecond},
		APIKeyRate: RateConfig{Limit: 100, Period: time.Minute},
		RouteRates: map[string]RateConfig{},
		Prefix:     "test:ratelimit:",
		MaxRetry:   1,
	}
	r := buildRouterForTest(t, cfg)

	res1 := doReq(r, "5.6.7.8")
	require.Equal(t, 200, res1.Code)
	res2 := doReq(r, "5.6.7.8")
	require.Equal(t, 429, res2.Code)
	// Wait for refill and try again
	time.Sleep(120 * time.Millisecond)
	res3 := doReq(r, "5.6.7.8")
	require.Equal(t, 200, res3.Code)
}

func TestInMemoryRateLimit_SetsHeaders(t *testing.T) {
	cfg := &Config{
		GlobalRate: RateConfig{Limit: 2, Period: time.Minute},
		APIKeyRate: RateConfig{Limit: 100, Period: time.Minute},
		RouteRates: map[string]RateConfig{},
		Prefix:     "test:ratelimit:",
		MaxRetry:   1,
	}
	r := buildRouterForTest(t, cfg)
	res := doReq(r, "9.9.9.9")
	require.Equal(t, 200, res.Code)
	require.NotEmpty(t, res.Header().Get("X-RateLimit-Limit"))
	require.NotEmpty(t, res.Header().Get("X-RateLimit-Remaining"))
	require.NotEmpty(t, res.Header().Get("X-RateLimit-Reset"))
}
