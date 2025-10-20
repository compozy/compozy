// Package ratelimit provides HTTP rate limiting middleware for Gin.
//
// The middleware supports configurable fail-open/fail-closed behavior when the backing
// store (Redis or in-memory) encounters errors. By default, it fails open to prioritize
// availability - requests are allowed through when rate limit checks fail. This can be
// changed to fail-closed mode via configuration to prioritize strict rate limiting
// enforcement over availability.
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
	"go.opentelemetry.io/otel/metric"
)

// Manager manages rate limiting for the application
type Manager struct {
	config        *Config
	store         limiter.Store
	limiters      map[string]*limiter.Limiter
	mu            sync.RWMutex
	globalLimiter *limiter.Limiter
	apiKeyLimiter *limiter.Limiter
	meter         metric.Meter
}

// NewManager creates a new rate limiting manager
func NewManager(config *Config, redisClient *redis.Client) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}
	storeOptions := limiter.StoreOptions{
		Prefix:   config.Prefix,
		MaxRetry: config.MaxRetry,
	}
	var store limiter.Store
	var err error
	if redisClient != nil {
		store, err = sredis.NewStoreWithOptions(redisClient, storeOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis store: %w", err)
		}
	} else {
		store = memory.NewStore()
	}
	globalLimiter := limiter.New(store, config.GlobalRate.ToLimiterRate())
	apiKeyLimiter := limiter.New(store, config.APIKeyRate.ToLimiterRate())
	m := &Manager{
		config:        config,
		store:         store,
		limiters:      make(map[string]*limiter.Limiter),
		globalLimiter: globalLimiter,
		apiKeyLimiter: apiKeyLimiter,
	}
	for route, rateConfig := range config.RouteRates {
		if !rateConfig.Disabled {
			m.limiters[route] = limiter.New(store, rateConfig.ToLimiterRate())
		}
	}
	return m, nil
}

// NewManagerWithMetrics creates a new rate limiting manager with metrics support
func NewManagerWithMetrics(
	ctx context.Context,
	config *Config,
	redisClient *redis.Client,
	meter metric.Meter,
) (*Manager, error) {
	m, err := NewManager(config, redisClient)
	if err != nil {
		return nil, err
	}
	m.meter = meter
	if meter != nil {
		if err := InitMetrics(meter); err != nil {
			log := logger.FromContext(ctx)
			log.Error("Failed to initialize rate limit metrics", "error", err)
		}
	}
	return m, nil
}

// Middleware returns the rate limiting middleware for Gin
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.isRequestExcluded(c) {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		key := m.keyGetter(c)
		limiter := m.getLimiterForRequest(c, key)

		lctx, err := limiter.Get(c.Request.Context(), key)
		if err != nil {
			log := logger.FromContext(c.Request.Context())
			keyType := m.getKeyType(key)
			log.Error("Rate limit check failed", "error", err, "key_type", keyType, "fail_open", m.config.FailOpen)
			if m.config.FailOpen {
				c.Next()
			} else {
				c.JSON(500, gin.H{
					"error":   "Internal server error",
					"details": "Rate limiting backend unavailable",
				})
				c.Abort()
			}
			return
		}

		m.setRateLimitHeaders(c, lctx)

		if lctx.Reached {
			m.handleRateLimitExceeded(c, lctx, path, key)
			return
		}

		c.Next()
	}
}

// isRequestExcluded checks if the request should be excluded from rate limiting
func (m *Manager) isRequestExcluded(c *gin.Context) bool {
	path := c.Request.URL.Path
	for _, excluded := range m.config.ExcludedPaths {
		if strings.HasPrefix(path, excluded) {
			return true
		}
	}
	clientIP := c.ClientIP()
	return slices.Contains(m.config.ExcludedIPs, clientIP)
}

// getLimiterForRequest returns the appropriate limiter for the request
func (m *Manager) getLimiterForRequest(c *gin.Context, key string) *limiter.Limiter {
	path := c.Request.URL.Path
	routeLimiter := m.getRouteSpecificLimiter(path)
	switch {
	case routeLimiter != nil:
		return routeLimiter
	case strings.HasPrefix(key, "apikey:"):
		return m.apiKeyLimiter
	default:
		return m.globalLimiter
	}
}

// setRateLimitHeaders sets the rate limit headers if not disabled
func (m *Manager) setRateLimitHeaders(c *gin.Context, lctx limiter.Context) {
	if !m.config.DisableHeaders {
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", lctx.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", lctx.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", lctx.Reset))
		resetIn := max(lctx.Reset-time.Now().Unix(), 0)
		c.Header("RateLimit-Limit", fmt.Sprintf("%d", lctx.Limit))
		if lctx.Remaining < 0 {
			c.Header("RateLimit-Remaining", "0")
		} else {
			c.Header("RateLimit-Remaining", fmt.Sprintf("%d", lctx.Remaining))
		}
		c.Header("RateLimit-Reset", fmt.Sprintf("%d", resetIn))
	}
}

// handleRateLimitExceeded handles the rate limit exceeded case
func (m *Manager) handleRateLimitExceeded(c *gin.Context, lctx limiter.Context, path, key string) {
	keyType := m.getKeyType(key)
	IncrementBlockedRequests(c.Request.Context(), path, keyType)
	resetIn := max(lctx.Reset-time.Now().Unix(), 0)
	detail := fmt.Sprintf("API rate limit exceeded. Retry after %d seconds", resetIn)
	c.Header("Retry-After", fmt.Sprintf("%d", resetIn))
	router.RespondProblem(c, &core.Problem{
		Status:   http.StatusTooManyRequests,
		Title:    "Too Many Requests",
		Detail:   detail,
		Type:     "https://docs.compozy.com/problems/rate-limit-exceeded",
		Instance: c.FullPath(),
		Extras:   map[string]any{"retry_after": resetIn},
	})
	c.Abort()
}

// getRouteSpecificLimiter returns a route-specific limiter if one exists, nil otherwise
func (m *Manager) getRouteSpecificLimiter(path string) *limiter.Limiter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var bestMatch string
	var bestLimiter *limiter.Limiter
	if limiter, exists := m.limiters[path]; exists {
		return limiter
	}
	for route, l := range m.limiters {
		if strings.HasPrefix(path, route) && len(route) > len(bestMatch) {
			bestMatch = route
			bestLimiter = l
		}
	}
	return bestLimiter
}

// keyGetter generates the rate limiting key based on user authentication
func (m *Manager) keyGetter(c *gin.Context) string {
	if apiKey, exists := c.Get(auth.ContextKeyAPIKey); exists {
		return fmt.Sprintf("apikey:%v", apiKey)
	}
	if userID, exists := c.Get(auth.ContextKeyUserID); exists {
		return fmt.Sprintf("user:%v", userID)
	}
	realIP := c.GetHeader("X-Real-IP")
	if realIP != "" {
		return fmt.Sprintf("ip:%s", realIP)
	}
	forwardedFor := c.GetHeader("X-Forwarded-For")
	if forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			return fmt.Sprintf("ip:%s", strings.TrimSpace(ips[0]))
		}
	}
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

// getKeyType extracts the type of key from the rate limit key
func (m *Manager) getKeyType(key string) string {
	switch {
	case strings.HasPrefix(key, "apikey:"):
		return "apikey"
	case strings.HasPrefix(key, "user:"):
		return "user"
	case strings.HasPrefix(key, "ip:"):
		return "ip"
	default:
		return "unknown"
	}
}

// UpdateRouteLimit updates the rate limit for a specific route
func (m *Manager) UpdateRouteLimit(ctx context.Context, route string, rateConfig RateConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rateConfig.Disabled {
		delete(m.limiters, route)
	} else {
		m.limiters[route] = limiter.New(m.store, rateConfig.ToLimiterRate())
	}
	log := logger.FromContext(ctx)
	log.Info("Updated rate limit for route", "route", route, "limit", rateConfig.Limit, "period", rateConfig.Period)
}

// GetLimitInfo returns current limit information for a key
func (m *Manager) GetLimitInfo(c *gin.Context) (*limiter.Context, error) {
	key := m.keyGetter(c)
	l := m.getLimiterForRequest(c, key)
	ctx, err := l.Peek(c.Request.Context(), key)
	if err != nil {
		return nil, err
	}
	return &ctx, nil
}
