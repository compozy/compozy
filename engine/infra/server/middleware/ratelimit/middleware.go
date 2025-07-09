package ratelimit

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/memory"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
)

// Manager manages rate limiting for the application
type Manager struct {
	config        *Config
	store         limiter.Store
	limiters      map[string]*limiter.Limiter
	mu            sync.RWMutex
	globalLimiter *limiter.Limiter
	log           logger.Logger
}

// NewManager creates a new rate limiting manager
func NewManager(config *Config, redisClient *redis.Client) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create logger with context
	ctx := context.Background()
	log := logger.FromContext(ctx).With("component", "ratelimit")

	// Create Redis store with options
	storeOptions := limiter.StoreOptions{
		Prefix:   config.Prefix,
		MaxRetry: config.MaxRetry,
	}

	// Use Redis if client provided, otherwise use in-memory store
	var store limiter.Store
	var err error

	if redisClient != nil {
		store, err = sredis.NewStoreWithOptions(redisClient, storeOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis store: %w", err)
		}
		log.Info("Rate limiting using Redis store", "addr", config.RedisAddr)
	} else {
		store = memory.NewStore()
		log.Warn("Rate limiting using in-memory store (not suitable for distributed systems)")
	}

	// Create global limiter
	globalLimiter := limiter.New(store, config.GlobalRate.ToLimiterRate())

	// Create manager
	m := &Manager{
		config:        config,
		store:         store,
		limiters:      make(map[string]*limiter.Limiter),
		globalLimiter: globalLimiter,
		log:           log,
	}

	// Pre-create route-specific limiters
	for route, rateConfig := range config.RouteRates {
		if !rateConfig.Disabled {
			m.limiters[route] = limiter.New(store, rateConfig.ToLimiterRate())
		}
	}

	return m, nil
}

// Middleware returns the rate limiting middleware for Gin
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if path is excluded
		path := c.Request.URL.Path
		for _, excluded := range m.config.ExcludedPaths {
			if strings.HasPrefix(path, excluded) {
				c.Next()
				return
			}
		}

		// Get limiter for this route
		limiter := m.getLimiterForRoute(path)

		// Apply rate limiting using the appropriate middleware
		middleware := mgin.NewMiddleware(limiter,
			mgin.WithKeyGetter(m.keyGetter),
			mgin.WithExcludedKey(m.isExcludedKey),
		)

		middleware(c)
	}
}

// MiddlewareForRoute returns rate limiting middleware for a specific route pattern
func (m *Manager) MiddlewareForRoute(routePattern string, rateConfig RateConfig) gin.HandlerFunc {
	// Create a specific limiter for this route if not exists
	m.mu.Lock()
	if _, exists := m.limiters[routePattern]; !exists {
		m.limiters[routePattern] = limiter.New(m.store, rateConfig.ToLimiterRate())
	}
	m.mu.Unlock()

	return func(c *gin.Context) {
		// Apply rate limiting for this specific route
		limiter := m.limiters[routePattern]
		middleware := mgin.NewMiddleware(limiter,
			mgin.WithKeyGetter(m.keyGetter),
			mgin.WithExcludedKey(m.isExcludedKey),
		)
		middleware(c)
	}
}

// getLimiterForRoute returns the appropriate limiter for a given route
func (m *Manager) getLimiterForRoute(path string) *limiter.Limiter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check for exact match first
	if limiter, exists := m.limiters[path]; exists {
		return limiter
	}

	// Check for prefix match (e.g., /api/v0/memory matches /api/v0/memory/:ref/:key)
	for route, limiter := range m.limiters {
		if strings.HasPrefix(path, route) {
			return limiter
		}
	}

	// Default to global limiter
	return m.globalLimiter
}

// keyGetter generates the rate limiting key based on user authentication
func (m *Manager) keyGetter(c *gin.Context) string {
	// If user is authenticated, use user ID
	if userID, exists := c.Get("userID"); exists {
		return fmt.Sprintf("user:%v", userID)
	}

	// For anonymous users, use IP address
	// Handle X-Real-IP and X-Forwarded-For headers for proxy scenarios
	realIP := c.GetHeader("X-Real-IP")
	if realIP != "" {
		return fmt.Sprintf("ip:%s", realIP)
	}

	forwardedFor := c.GetHeader("X-Forwarded-For")
	if forwardedFor != "" {
		// Use the first IP in the chain
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			return fmt.Sprintf("ip:%s", strings.TrimSpace(ips[0]))
		}
	}

	// Fallback to client IP
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

// isExcludedKey determines if a key should be excluded from rate limiting
func (m *Manager) isExcludedKey(key string) bool {
	// - **Example**: exclude certain admin users or internal services
	// This can be extended based on requirements
	return strings.HasPrefix(key, "internal:")
}

// UpdateRouteLimit updates the rate limit for a specific route
func (m *Manager) UpdateRouteLimit(route string, rateConfig RateConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rateConfig.Disabled {
		delete(m.limiters, route)
	} else {
		m.limiters[route] = limiter.New(m.store, rateConfig.ToLimiterRate())
	}

	m.log.Info("Updated rate limit for route", "route", route, "limit", rateConfig.Limit, "period", rateConfig.Period)
}

// GetLimitInfo returns current limit information for a key
func (m *Manager) GetLimitInfo(c *gin.Context) (*limiter.Context, error) {
	key := m.keyGetter(c)
	path := c.Request.URL.Path
	l := m.getLimiterForRoute(path)

	ctx, err := l.Peek(c.Request.Context(), key)
	if err != nil {
		return nil, err
	}
	return &ctx, nil
}
