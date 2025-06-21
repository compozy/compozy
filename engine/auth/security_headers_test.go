package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersMiddleware_AddSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should add default security headers", func(t *testing.T) {
		// Setup with default config
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert security headers are set
		headers := w.Header()
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		assert.Equal(t, "0", headers.Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
		assert.Equal(t, "geolocation=(), microphone=(), camera=()", headers.Get("Permissions-Policy"))
		assert.Equal(t, "", headers.Get("Server"))
		// Default config should enable HSTS and CSP
		assert.Contains(t, headers.Get("Strict-Transport-Security"), "max-age=31536000")
		assert.Contains(t, headers.Get("Strict-Transport-Security"), "includeSubDomains")
		assert.Contains(t, headers.Get("Content-Security-Policy"), "default-src 'self'")
	})
	t.Run("Should respect custom configuration", func(t *testing.T) {
		// Setup with custom config
		config := &SecurityHeadersConfig{
			EnableHSTS:            false,
			EnableCSP:             false,
			HSTSMaxAge:            "3600",
			ContentSecurityPolicy: "default-src 'none'",
		}
		middleware := NewSecurityHeadersMiddleware(config)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert custom configuration is applied
		headers := w.Header()
		assert.Empty(t, headers.Get("Strict-Transport-Security"))
		assert.Empty(t, headers.Get("Content-Security-Policy"))
	})
	t.Run("Should add custom HSTS with custom max-age", func(t *testing.T) {
		// Setup with custom HSTS
		config := &SecurityHeadersConfig{
			EnableHSTS:            true,
			HSTSMaxAge:            "7200",
			EnableCSP:             false,
			ContentSecurityPolicy: "",
		}
		middleware := NewSecurityHeadersMiddleware(config)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert custom HSTS is set
		headers := w.Header()
		assert.Equal(t, "max-age=7200; includeSubDomains", headers.Get("Strict-Transport-Security"))
		assert.Empty(t, headers.Get("Content-Security-Policy"))
	})
	t.Run("Should add custom CSP", func(t *testing.T) {
		// Setup with custom CSP
		config := &SecurityHeadersConfig{
			EnableHSTS:            false,
			EnableCSP:             true,
			ContentSecurityPolicy: "default-src 'none'; script-src 'self'",
		}
		middleware := NewSecurityHeadersMiddleware(config)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert custom CSP is set
		headers := w.Header()
		assert.Empty(t, headers.Get("Strict-Transport-Security"))
		assert.Equal(t, "default-src 'none'; script-src 'self'", headers.Get("Content-Security-Policy"))
	})
	t.Run("Should add cache control headers for authenticated endpoints", func(t *testing.T) {
		// Setup
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Add API key to context to simulate authenticated endpoint
		ctx := c.Request.Context()
		testKey := &apikey.APIKey{ID: core.MustNewID()}
		ctx = WithAPIKey(ctx, testKey)
		c.Request = c.Request.WithContext(ctx)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert cache control headers are set for authenticated endpoints
		headers := w.Header()
		assert.Equal(t, "no-store, no-cache, must-revalidate, private", headers.Get("Cache-Control"))
		assert.Equal(t, "no-cache", headers.Get("Pragma"))
		assert.Equal(t, "0", headers.Get("Expires"))
	})
	t.Run("Should add cache control headers for POST requests", func(t *testing.T) {
		// Setup
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert cache control headers are set for non-GET requests
		headers := w.Header()
		assert.Equal(t, "no-store, no-cache, must-revalidate, private", headers.Get("Cache-Control"))
		assert.Equal(t, "no-cache", headers.Get("Pragma"))
		assert.Equal(t, "0", headers.Get("Expires"))
	})
	t.Run("Should not add cache control headers for public GET endpoints", func(t *testing.T) {
		// Setup
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Do not add API key to context (public endpoint)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert cache control headers are NOT set for public GET endpoints
		headers := w.Header()
		assert.Empty(t, headers.Get("Cache-Control"))
		assert.Empty(t, headers.Get("Pragma"))
		assert.Empty(t, headers.Get("Expires"))
	})
	t.Run("Should completely remove Server header", func(t *testing.T) {
		// Setup
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Pre-set a Server header to verify it gets removed
		w.Header().Set("Server", "nginx/1.20.1")
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert Server header is completely removed (not just set to empty)
		headers := w.Header()
		_, exists := headers["Server"]
		assert.False(t, exists, "Server header should be completely removed, not just empty")
		assert.Empty(t, headers.Get("Server"))
	})
	t.Run("Should set all security headers for authenticated endpoints", func(t *testing.T) {
		// Setup authenticated endpoint
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/users", http.NoBody)
		// Add API key to simulate authenticated endpoint
		ctx := c.Request.Context()
		testKey := &apikey.APIKey{ID: core.MustNewID()}
		ctx = WithAPIKey(ctx, testKey)
		c.Request = c.Request.WithContext(ctx)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Verify ALL security headers are present
		headers := w.Header()
		// Core security headers
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		assert.Equal(t, "0", headers.Get("X-XSS-Protection"))
		assert.Equal(t, "strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))
		assert.Equal(t, "geolocation=(), microphone=(), camera=()", headers.Get("Permissions-Policy"))
		// HSTS should be enabled by default
		hstsHeader := headers.Get("Strict-Transport-Security")
		assert.Contains(t, hstsHeader, "max-age=31536000")
		assert.Contains(t, hstsHeader, "includeSubDomains")
		// CSP should be enabled by default
		cspHeader := headers.Get("Content-Security-Policy")
		assert.Contains(t, cspHeader, "default-src 'self'")
		// Server header should be removed
		_, serverExists := headers["Server"]
		assert.False(t, serverExists)
		// Cache control headers for authenticated endpoints
		assert.Equal(t, "no-store, no-cache, must-revalidate, private", headers.Get("Cache-Control"))
		assert.Equal(t, "no-cache", headers.Get("Pragma"))
		assert.Equal(t, "0", headers.Get("Expires"))
	})
	t.Run("Should handle middleware chain correctly", func(t *testing.T) {
		// Setup middleware chain
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, engine := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Set up a full middleware chain
		engine.Use(middleware.AddSecurityHeaders())
		engine.GET("/test", func(c *gin.Context) {
			// Handler should be able to add its own headers
			c.Header("X-Custom-Header", "custom-value")
			c.JSON(200, gin.H{"message": "success"})
		})
		// Execute the full chain
		engine.ServeHTTP(w, c.Request)
		// Verify security headers are set
		headers := w.Header()
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		// Verify custom handler header is also set
		assert.Equal(t, "custom-value", headers.Get("X-Custom-Header"))
		// Verify response is correct
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})
	t.Run("Should respect X-XSS-Protection modern setting", func(t *testing.T) {
		// Setup
		middleware := NewSecurityHeadersMiddleware(nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.AddSecurityHeaders()
		handler(c)
		// Assert X-XSS-Protection is set to "0" (modern best practice)
		headers := w.Header()
		assert.Equal(
			t,
			"0",
			headers.Get("X-XSS-Protection"),
			"X-XSS-Protection should be '0' to disable XSS auditor and rely on CSP",
		)
	})
}

func TestDefaultSecurityHeadersConfig(t *testing.T) {
	t.Run("Should return sensible defaults", func(t *testing.T) {
		config := DefaultSecurityHeadersConfig()
		assert.NotNil(t, config)
		assert.True(t, config.EnableHSTS)
		assert.Equal(t, "31536000", config.HSTSMaxAge)
		assert.True(t, config.EnableCSP)
		assert.Contains(t, config.ContentSecurityPolicy, "default-src 'self'")
		assert.Contains(t, config.ContentSecurityPolicy, "object-src 'none'")
	})
}

func Test_isAuthenticatedEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should return true when API key exists in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Add API key to context
		ctx := c.Request.Context()
		testKey := &apikey.APIKey{ID: core.MustNewID()}
		ctx = WithAPIKey(ctx, testKey)
		c.Request = c.Request.WithContext(ctx)
		result := isAuthenticatedEndpoint(c)
		assert.True(t, result)
	})
	t.Run("Should return false when no API key in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		result := isAuthenticatedEndpoint(c)
		assert.False(t, result)
	})
}
