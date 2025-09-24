package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	ginmode "github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildRouterForTest(t *testing.T, cfg *Config) *gin.Engine {
	t.Helper()
	ginmode.EnsureGinTestMode()
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
	t.Run("Should block second request when global limit is 1 per second", func(t *testing.T) {
		cfg := &Config{
			GlobalRate: RateConfig{Limit: 1, Period: time.Second},
			APIKeyRate: RateConfig{Limit: 100, Period: time.Minute},
			RouteRates: map[string]RateConfig{},
			Prefix:     "test:ratelimit:",
			MaxRetry:   1,
		}
		r := buildRouterForTest(t, cfg)
		res1 := doReq(r, "1.2.3.4")
		require.Equal(t, http.StatusOK, res1.Code)
		res2 := doReq(r, "1.2.3.4")
		require.Equal(t, http.StatusTooManyRequests, res2.Code)
	})
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
	require.Equal(t, http.StatusOK, res1.Code)
	res2 := doReq(r, "5.6.7.8")
	require.Equal(t, http.StatusTooManyRequests, res2.Code)
	// Wait for refill and try again
	time.Sleep(120 * time.Millisecond)
	res3 := doReq(r, "5.6.7.8")
	require.Equal(t, http.StatusOK, res3.Code)
}

func TestInMemoryRateLimit_SetsHeaders(t *testing.T) {
	t.Run("Should set rate-limit headers on success", func(t *testing.T) {
		cfg := &Config{
			GlobalRate: RateConfig{Limit: 2, Period: time.Minute},
			APIKeyRate: RateConfig{Limit: 100, Period: time.Minute},
			RouteRates: map[string]RateConfig{},
			Prefix:     "test:ratelimit:",
			MaxRetry:   1,
		}
		r := buildRouterForTest(t, cfg)
		res := doReq(r, "9.9.9.9")
		require.Equal(t, http.StatusOK, res.Code)
		limitHeader := res.Header().Get("X-RateLimit-Limit")
		require.NotEmpty(t, limitHeader)
		limit, err := strconv.ParseInt(limitHeader, 10, 64)
		require.NoError(t, err)
		require.Equal(t, cfg.GlobalRate.Limit, limit)
		remainingHeader := res.Header().Get("X-RateLimit-Remaining")
		require.NotEmpty(t, remainingHeader)
		remaining, err := strconv.ParseInt(remainingHeader, 10, 64)
		require.NoError(t, err)
		require.Equal(t, cfg.GlobalRate.Limit-1, remaining)
		resetHeader := res.Header().Get("X-RateLimit-Reset")
		require.NotEmpty(t, resetHeader)
		reset, err := strconv.ParseInt(resetHeader, 10, 64)
		require.NoError(t, err)
		now := time.Now().Unix()
		require.GreaterOrEqual(t, reset, now)
		maxReset := now + int64(cfg.GlobalRate.Period.Seconds()) + 1
		require.LessOrEqual(t, reset, maxReset)
	})
}
