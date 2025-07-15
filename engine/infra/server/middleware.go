package server

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// LoggerMiddleware logs HTTP request details.
func LoggerMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
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
		if raw != "" {
			path = path + "?" + raw
		}
		param.Path = path
		ctx := logger.ContextWithLogger(c.Request.Context(), log)
		log := logger.FromContext(ctx)
		log.Info("Request completed",
			"timestamp", param.TimeStamp.Format(time.RFC3339),
			"latency", param.Latency,
			"client_ip", param.ClientIP,
			"method", param.Method,
			"status_code", param.StatusCode,
			"body_size", param.BodySize,
			"path", param.Path,
			"error", param.ErrorMessage,
		)
	}
}

// CORSMiddleware enables CORS support with configurable origins.
func CORSMiddleware(corsConfig config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		isAllowed := false
		if len(corsConfig.AllowedOrigins) == 0 {
			// If no origins configured, allow none (secure default)
			isAllowed = false
		} else {
			for _, allowed := range corsConfig.AllowedOrigins {
				if origin == allowed {
					isAllowed = true
					break
				}
			}
		}

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
