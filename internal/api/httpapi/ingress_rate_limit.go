package httpapi

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/gin-gonic/gin"
)

func (h *Handlers) ingressRateLimitMiddleware() gin.HandlerFunc {
	if h == nil {
		return ingressRateLimit(nil)
	}
	return ingressRateLimit(h.ingressLimiter)
}

func ingressRateLimit(limiter *ingress.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil || c.Request == nil || c.Request.URL == nil {
			c.Next()
			return
		}
		allowed, retryAfter := limiter.Allow(
			strings.TrimSpace(c.Request.URL.Path),
			requestSourceIP(c.Request),
		)
		if allowed {
			c.Next()
			return
		}
		seconds := max(int64(math.Ceil(retryAfter.Seconds())), int64(1))
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
		core.RespondError(c, http.StatusTooManyRequests, ingress.ErrRateLimited, false)
		c.Abort()
	}
}
