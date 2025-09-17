package corsmiddleware

import (
	"net/http"
	"strconv"

	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

// Middleware applies basic CORS headers based on configuration.
func Middleware(cfg config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := false
		for _, allowedOrigin := range cfg.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}
		if allowed && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			reqHdr := c.Request.Header.Get("Access-Control-Request-Headers")
			if reqHdr != "" {
				c.Header("Access-Control-Allow-Headers", reqHdr)
			} else {
				c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
			if cfg.MaxAge > 0 {
				c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}
		}
		if c.Request.Method == http.MethodOptions {
			if allowed && origin != "" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}
