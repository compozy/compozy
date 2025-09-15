package server

import (
	"context"

	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

// LoggerMiddleware attaches request-scoped logger metadata. Minimal no-op for greenfield cleanup.
func LoggerMiddleware(_ context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Placeholder for request logging enrichment; proceed to next handler
		c.Next()
	}
}

// CORSMiddleware applies basic CORS headers based on configuration. Minimal pass-through here.
func CORSMiddleware(_ config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// In production, set Access-Control-* headers based on cfg.
		// This simplified middleware preserves previous call sites without behavior changes.
		c.Next()
	}
}
