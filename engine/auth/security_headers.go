package auth

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds security headers to all responses
type SecurityHeadersMiddleware struct {
	// Configuration options
	enableHSTS            bool
	hstsMaxAge            string
	enableCSP             bool
	contentSecurityPolicy string
}

// SecurityHeadersConfig holds configuration for security headers
type SecurityHeadersConfig struct {
	// Enable HTTP Strict Transport Security
	EnableHSTS bool
	// HSTS max age in seconds (default: 31536000 - 1 year)
	HSTSMaxAge string
	// Enable Content Security Policy
	EnableCSP bool
	// Content Security Policy directives
	ContentSecurityPolicy string
}

// DefaultSecurityHeadersConfig returns default security headers configuration
func DefaultSecurityHeadersConfig() *SecurityHeadersConfig {
	return &SecurityHeadersConfig{
		EnableHSTS:            true,
		HSTSMaxAge:            "31536000", // 1 year
		EnableCSP:             true,
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; object-src 'none'; frame-ancestors 'none';",
	}
}

// NewSecurityHeadersMiddleware creates a new security headers middleware
func NewSecurityHeadersMiddleware(config *SecurityHeadersConfig) *SecurityHeadersMiddleware {
	if config == nil {
		config = DefaultSecurityHeadersConfig()
	}
	return &SecurityHeadersMiddleware{
		enableHSTS:            config.EnableHSTS,
		hstsMaxAge:            config.HSTSMaxAge,
		enableCSP:             config.EnableCSP,
		contentSecurityPolicy: config.ContentSecurityPolicy,
	}
}

// AddSecurityHeaders is the Gin middleware handler that adds security headers
func (m *SecurityHeadersMiddleware) AddSecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// X-Content-Type-Options: Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		// X-Frame-Options: Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")
		// X-XSS-Protection: Disable XSS auditor, rely on CSP instead
		c.Header("X-XSS-Protection", "0")
		// Referrer-Policy: Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		// Permissions-Policy: Control browser features
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// HTTP Strict Transport Security (HSTS)
		if m.enableHSTS {
			hstsValue := "max-age=" + m.hstsMaxAge + "; includeSubDomains"
			c.Header("Strict-Transport-Security", hstsValue)
		}
		// Content-Security-Policy
		if m.enableCSP {
			c.Header("Content-Security-Policy", m.contentSecurityPolicy)
		}
		// Remove server header to avoid exposing server information
		c.Header("Server", "")
		// Add cache control for sensitive endpoints
		if c.Request.Method != "GET" || isAuthenticatedEndpoint(c) {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	}
}

// isAuthenticatedEndpoint checks if the current endpoint requires authentication
func isAuthenticatedEndpoint(c *gin.Context) bool {
	// Check if API key exists in context (set by AuthMiddleware)
	_, exists := GetAPIKey(c)
	return exists
}
