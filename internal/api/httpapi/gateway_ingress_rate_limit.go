package httpapi

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/gin-gonic/gin"
)

func (h *Handlers) gatewayIngressRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.ingressLimiter == nil || c.Request == nil || c.Request.URL == nil {
			c.Next()
			return
		}
		allowed, retryAfter := h.ingressLimiter.Allow(
			strings.TrimSpace(c.Request.URL.Path),
			gatewayAuthSource(c.Request),
		)
		if allowed {
			c.Next()
			return
		}
		seconds := max(int64(math.Ceil(retryAfter.Seconds())), int64(1))
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
		core.RespondError(c, http.StatusTooManyRequests, gateway.ErrIngressRateLimited, false)
		c.Abort()
	}
}
