package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// LoggerMiddleware attaches request-scoped logger metadata.
func LoggerMiddleware(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		mgr := config.ManagerFromContext(ctx)
		log := logger.FromContext(ctx)
		reqCtx := c.Request.Context()
		reqCtx = config.ContextWithManager(reqCtx, mgr)
		reqCtx = logger.ContextWithLogger(reqCtx, log)
		c.Request = c.Request.WithContext(reqCtx)
		c.Next()
	}
}

// CORSMiddleware applies basic CORS headers based on configuration.
func CORSMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
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
			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if cfg.MaxAge > 0 {
				c.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
