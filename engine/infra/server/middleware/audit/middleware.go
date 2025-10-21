package audit

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// sanitizeQuery removes sensitive parameters from query strings before logging
func sanitizeQuery(values url.Values) map[string][]string {
	sanitized := make(map[string][]string)
	sensitiveParams := map[string]bool{
		"api_key":       true,
		"apikey":        true,
		"token":         true,
		"access_token":  true,
		"auth_token":    true,
		"bearer":        true,
		"password":      true,
		"pwd":           true,
		"secret":        true,
		"key":           true,
		"oauth_token":   true,
		"refresh_token": true,
		"session_id":    true,
		"csrf_token":    true,
	}
	for key, vals := range values {
		lowerKey := strings.ToLower(key)
		if sensitiveParams[lowerKey] || strings.Contains(lowerKey, "token") || strings.Contains(lowerKey, "key") ||
			strings.Contains(lowerKey, "secret") {
			sanitized[key] = []string{"[REDACTED]"}
		} else {
			sanitized[key] = vals
		}
	}
	return sanitized
}

// getCachedLogger retrieves the cached logger from gin context or falls back to context logger
func getCachedLogger(c *gin.Context) logger.Logger {
	if cachedLogger, exists := c.Get("audit_logger"); exists {
		if log, ok := cachedLogger.(logger.Logger); ok {
			return log
		}
	}
	return logger.FromContext(c.Request.Context())
}

// formatUserID safely formats user ID for logging, handling any type
func formatUserID(userID any) string {
	if userID == nil {
		return ""
	}
	return fmt.Sprintf("%v", userID)
}

// SecurityAuditMiddleware creates a middleware for comprehensive security audit logging
func SecurityAuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		log := logger.FromContext(c.Request.Context())
		c.Set("audit_logger", log)
		logRequestDetails(c, start)
		c.Next()
		logResponseDetails(c, start)
	}
}

// logRequestDetails logs incoming request information
func logRequestDetails(c *gin.Context, start time.Time) {
	log := getCachedLogger(c)
	userID, userExists := c.Get("user_id")
	log.Info("security_audit_request",
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"query", sanitizeQuery(c.Request.URL.Query()),
		"client_ip", c.ClientIP(),
		"user_agent", c.Request.UserAgent(),
		"has_auth", c.GetHeader("Authorization") != "",
		"user_id", formatUserID(userID),
		"user_exists", userExists,
		"timestamp", start.Format(time.RFC3339),
	)
}

// logResponseDetails logs response information with security audit details
func logResponseDetails(c *gin.Context, start time.Time) {
	duration := time.Since(start)
	status := c.Writer.Status()
	userID, userExists := c.Get("user_id")
	logResponseBySeverity(c, duration, status, userID, userExists)
	logAuthFailures(c, status, userID)
}

// buildResponseLog creates a structured response log with all relevant information
func buildResponseLog(
	c *gin.Context,
	duration time.Duration,
	status int,
	userID any,
	userExists bool,
) logger.Logger {
	log := getCachedLogger(c)
	responseLog := log.With(
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"response_size", c.Writer.Size(),
		"client_ip", c.ClientIP(),
		"user_id", formatUserID(userID),
		"user_exists", userExists,
		"timestamp", time.Now().Format(time.RFC3339),
	)
	if isSecuritySensitiveEndpoint(c.Request.URL.Path) {
		responseLog = responseLog.With("security_sensitive", true)
	}
	if workflowID := c.Param("workflow_id"); workflowID != "" {
		responseLog = responseLog.With("workflow_id", workflowID)
	}
	if execID := c.Param("exec_id"); execID != "" {
		responseLog = responseLog.With("execution_id", execID)
	}
	return responseLog
}

// logResponseBySeverity logs response based on HTTP status code severity
func logResponseBySeverity(c *gin.Context, duration time.Duration, status int, userID any, userExists bool) {
	responseLog := buildResponseLog(c, duration, status, userID, userExists)
	switch {
	case status >= 500:
		responseLog.Error("security_audit_response_error")
	case status >= 400:
		responseLog.Warn("security_audit_response_warning")
	default:
		responseLog.Info("security_audit_response")
	}
}

// logAuthFailures logs authentication failures specifically
func logAuthFailures(c *gin.Context, status int, userID any) {
	log := getCachedLogger(c)
	if status == 401 || status == 403 {
		log.Warn("security_audit_auth_failure",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"has_auth", c.GetHeader("Authorization") != "",
			"user_id", formatUserID(userID),
			"timestamp", time.Now().Format(time.RFC3339),
		)
	}
}

// isSecuritySensitiveEndpoint checks if the endpoint is security-sensitive
// Uses precise matching to avoid false positives
func isSecuritySensitiveEndpoint(path string) bool {
	bases := []string{
		routes.Auth(),
		routes.Users(),
		routes.Executions(),
		routes.Workflows(),
		routes.Hooks(),
	}
	for _, b := range bases {
		if path == b || strings.HasPrefix(path, b+"/") {
			return true
		}
	}
	return false
}

// AuthenticationEventMiddleware logs specific authentication events
func AuthenticationEventMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := getCachedLogger(c)
		c.Next()
		if authEvent, exists := c.Get("auth_event"); exists {
			if event, ok := authEvent.(string); ok {
				log.Info("security_audit_auth_event",
					"event", event,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"client_ip", c.ClientIP(),
					"user_agent", c.Request.UserAgent(),
					"timestamp", time.Now().Format(time.RFC3339),
				)
			}
		}
		if userID, exists := c.Get("user_id"); exists {
			if userExists := c.GetBool("user_exists"); userExists {
				log.Info("security_audit_user_context",
					"user_id", formatUserID(userID),
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"client_ip", c.ClientIP(),
					"timestamp", time.Now().Format(time.RFC3339),
				)
			}
		}
	}
}

// RateLimitAuditMiddleware logs rate limiting events
func RateLimitAuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := getCachedLogger(c)
		c.Next()
		if rateLimitEvent, exists := c.Get("rate_limit_event"); exists {
			if event, ok := rateLimitEvent.(map[string]any); ok {
				log.Warn("security_audit_rate_limit",
					"event", event,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"client_ip", c.ClientIP(),
					"user_agent", c.Request.UserAgent(),
					"timestamp", time.Now().Format(time.RFC3339),
				)
			}
		}
		if remaining := c.GetHeader("X-RateLimit-Remaining"); remaining != "" {
			if remainingInt, err := strconv.Atoi(remaining); err == nil && remainingInt < 10 {
				log.Warn("security_audit_rate_limit_warning",
					"remaining_requests", remainingInt,
					"path", c.Request.URL.Path,
					"client_ip", c.ClientIP(),
					"timestamp", time.Now().Format(time.RFC3339),
				)
			}
		}
	}
}
