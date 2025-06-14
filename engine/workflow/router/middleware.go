package wfrouter

import (
	"context"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const authClaimsKey contextKey = "auth_claims"

// AuthClaims represents authentication claims from a bearer token
type AuthClaims struct {
	ProjectID   string   `json:"project_id"`
	Permissions []string `json:"permissions"`
}

// HasPermission checks if the claims contain a specific permission
func (c *AuthClaims) HasPermission(permission string) bool {
	for _, perm := range c.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// AuthMiddleware creates a middleware that validates Bearer tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}
		if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			c.JSON(401, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}
		token := strings.TrimSpace(authHeader[7:])
		if token == "" {
			c.JSON(401, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}
		// Validate token against environment variable
		expectedToken := os.Getenv("COMPOZY_API_TOKEN")
		if expectedToken == "" {
			// Fallback to development tokens if env var not set
			expectedToken = "dev-token"
		}
		var claims *AuthClaims
		if token == expectedToken {
			claims = &AuthClaims{
				ProjectID:   "default-project",
				Permissions: []string{"events:write"},
			}
		} else {
			c.JSON(401, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}
		// Add claims to context
		ctx := context.WithValue(c.Request.Context(), authClaimsKey, claims)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequirePermission creates middleware that checks for specific permissions
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claimsValue := c.Request.Context().Value(authClaimsKey)
		claims, ok := claimsValue.(*AuthClaims)
		if !ok || claims == nil {
			c.JSON(500, gin.H{"error": "Authentication context missing"})
			c.Abort()
			return
		}
		if !claims.HasPermission(permission) {
			c.JSON(403, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}
