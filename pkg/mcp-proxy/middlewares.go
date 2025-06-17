package mcpproxy

import (
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// combineAuthTokens combines global auth tokens with client-specific tokens
func combineAuthTokens(globalTokens, clientTokens []string) []string {
	if len(globalTokens) == 0 {
		return clientTokens
	}

	if len(clientTokens) == 0 {
		return globalTokens
	}

	// Combine both sets, avoiding duplicates
	combined := make([]string, 0, len(globalTokens)+len(clientTokens))
	tokenSet := make(map[string]struct{})

	// Add global tokens first
	for _, token := range globalTokens {
		if token == "" {
			continue // Skip empty tokens
		}
		if _, exists := tokenSet[token]; !exists {
			combined = append(combined, token)
			tokenSet[token] = struct{}{}
		}
	}

	// Add client tokens
	for _, token := range clientTokens {
		if token == "" {
			continue // Skip empty tokens
		}
		if _, exists := tokenSet[token]; !exists {
			combined = append(combined, token)
			tokenSet[token] = struct{}{}
		}
	}

	return combined
}

const bearerPrefix = "Bearer "

// wrapWithGinMiddlewares wraps an http.Handler with gin middlewares
func wrapWithGinMiddlewares(handler http.Handler, middlewares ...gin.HandlerFunc) gin.HandlerFunc {
	// Create a new gin engine to properly chain middlewares
	engine := gin.New()

	// Add all middlewares to the engine
	for _, middleware := range middlewares {
		engine.Use(middleware)
	}

	// Add the final handler that calls the wrapped http.Handler
	engine.Use(func(c *gin.Context) {
		if handler == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Handler not initialized"})
			return
		}
		handler.ServeHTTP(c.Writer, c.Request)
	})

	// Return a handler that uses the engine
	return func(c *gin.Context) {
		engine.HandleContext(c)
	}
}

// newAuthMiddleware creates an authentication middleware with given tokens
func newAuthMiddleware(tokens []string) gin.HandlerFunc {
	tokenSet := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if token == "" {
			continue // Skip empty tokens to prevent security issues
		}
		tokenSet[token] = struct{}{}
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		if len(authHeader) < len(bearerPrefix) || !strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		token := strings.TrimSpace(authHeader[len(bearerPrefix):])
		if _, valid := tokenSet[token]; !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// loggerMiddleware creates a logging middleware for requests

// recoverMiddleware creates a panic recovery middleware
func recoverMiddleware(clientName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.FromContext(c.Request.Context())
		defer func() {
			if r := recover(); r != nil {
				log.Error("Panic recovered in MCP proxy",
					"client", clientName,
					"panic", r,
					"stack", string(debug.Stack()))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				c.Abort()
			}
		}()
		c.Next()
	}
}
