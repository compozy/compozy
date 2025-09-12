package server

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// LoggerMiddleware logs HTTP request details.
func LoggerMiddleware(ctx context.Context) gin.HandlerFunc {
	log := logger.FromContext(ctx)
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		mgr := config.ManagerFromContext(ctx)
		reqCtx := config.ContextWithManager(c.Request.Context(), mgr)
		reqCtx = logger.ContextWithLogger(reqCtx, log)
		c.Request = c.Request.WithContext(reqCtx)

		c.Next()
		param := gin.LogFormatterParams{
			Request: c.Request,
			Keys:    c.Keys,
		}
		param.TimeStamp = time.Now()
		param.Latency = param.TimeStamp.Sub(start)
		param.ClientIP = c.ClientIP()
		param.Method = c.Request.Method
		param.StatusCode = c.Writer.Status()
		param.ErrorMessage = c.Errors.ByType(gin.ErrorTypePrivate).String()
		param.BodySize = c.Writer.Size()
		// Avoid logging raw query strings to prevent leaking secrets
		param.Path = path
		// Use the request-scoped logger for request completion log
		reqLog := logger.FromContext(c.Request.Context())
		reqLog.Info("Request completed",
			"timestamp", param.TimeStamp.Format(time.RFC3339),
			"latency", param.Latency,
			"client_ip", param.ClientIP,
			"method", param.Method,
			"status_code", param.StatusCode,
			"body_size", param.BodySize,
			"path", param.Path,
			"query_present", raw != "",
			"error", param.ErrorMessage,
		)
	}
}

// CORSMiddleware enables CORS support with configurable origins.
func CORSMiddleware(corsConfig config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		isAllowed := len(corsConfig.AllowedOrigins) > 0 && contains(corsConfig.AllowedOrigins, origin)

		if isAllowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			if corsConfig.AllowCredentials {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Writer.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, "+
				"Authorization, accept, origin, Cache-Control, X-Requested-With",
		)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if corsConfig.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", corsConfig.MaxAge))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
