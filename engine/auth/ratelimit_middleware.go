package auth

import (
	"github.com/compozy/compozy/engine/auth/ratelimit"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware handles rate limiting for API requests
type RateLimitMiddleware struct {
	rateLimitService *ratelimit.Service
}

// NewRateLimitMiddleware creates a new rate limit middleware instance
func NewRateLimitMiddleware(rateLimitService *ratelimit.Service) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		rateLimitService: rateLimitService,
	}
}

// LimitByAPIKey is the Gin middleware handler for API key-based rate limiting
func (m *RateLimitMiddleware) LimitByAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		// This middleware should run after AuthMiddleware
		apiKey, exists := GetAPIKey(c)
		if !exists {
			// If no API key is found, allow the request to proceed
			// This allows for public endpoints or other auth methods
			c.Next()
			return
		}
		// Check rate limit for this API key
		// TODO: In the future, we could fetch custom limits from the API key record
		// For now, use default limits from config
		err := m.rateLimitService.CheckRateLimit(c.Request.Context(), apiKey.ID, nil)
		if err != nil {
			log := logger.FromContext(c.Request.Context())
			if coreErr, ok := err.(*core.Error); ok && coreErr.Code == "RATE_LIMIT_EXCEEDED" {
				log.With(
					"api_key_id", apiKey.ID,
					"user_id", apiKey.UserID,
					"org_id", apiKey.OrgID,
				).Warn("Rate limit exceeded")
				SendRateLimitError(c, "API rate limit exceeded. Please try again later.")
				return
			}
			// Unexpected error
			log.With("error", err).Error("Rate limit check failed")
			SendInternalServerError(c, "Rate limiting service unavailable")
			return
		}
		c.Next()
	}
}

// LimitByIP is the Gin middleware handler for IP-based rate limiting
// This can be used for public endpoints or as an additional layer of protection
func (m *RateLimitMiddleware) LimitByIP(requestsPerSecond int, burst int) gin.HandlerFunc {
	// Create a custom config for IP-based limiting
	ipConfig := &ratelimit.Config{
		DefaultRequestsPerSecond: requestsPerSecond,
		DefaultBurstCapacity:     burst,
		CleanupInterval:          ratelimit.DefaultConfig().CleanupInterval,
		LimiterExpiry:            ratelimit.DefaultConfig().LimiterExpiry,
	}
	// Create the service ONCE, outside the request handler to avoid memory leak
	ipLimitService := ratelimit.NewService(ipConfig)
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		// Create a pseudo-ID for the IP address
		ipID := core.ID(clientIP)
		err := ipLimitService.CheckRateLimit(c.Request.Context(), ipID, nil)
		if err != nil {
			log := logger.FromContext(c.Request.Context())
			if coreErr, ok := err.(*core.Error); ok && coreErr.Code == "RATE_LIMIT_EXCEEDED" {
				log.With("client_ip", clientIP).Warn("Rate limit exceeded for IP")
				SendRateLimitError(c, "Rate limit exceeded. Please try again later.")
				return
			}
			// Unexpected error
			log.With("error", err).Error("IP rate limit check failed")
			SendInternalServerError(c, "Rate limiting service unavailable")
			return
		}
		c.Next()
	}
}
