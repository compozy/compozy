package auth

import (
	"context"
	"strings"

	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// APIKeyValidator interface for dependency injection
type APIKeyValidator interface {
	ValidateKey(ctx context.Context, keyStr string) (*apikey.APIKey, *user.User, *org.Organization, error)
}

// Middleware handles API key authentication for all protected routes
type Middleware struct {
	apiKeyService APIKeyValidator
}

// NewAuthMiddleware creates a new authentication middleware instance
func NewAuthMiddleware(apiKeyService APIKeyValidator) *Middleware {
	return &Middleware{
		apiKeyService: apiKeyService,
	}
}

// NewAuthMiddlewareWithService creates a new authentication middleware instance with the concrete service
func NewAuthMiddlewareWithService(apiKeyService *apikey.Service) *Middleware {
	return NewAuthMiddleware(apiKeyService)
}

// Authenticate is the Gin middleware handler for API key authentication
func (m *Middleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.FromContext(c.Request.Context())
		// Extract Bearer token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Debug("Missing Authorization header")
			SendUnauthorizedError(c, "Missing Authorization header")
			return
		}
		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			log.Debug("Invalid Authorization header format")
			SendUnauthorizedError(c, "Invalid Authorization header format. Expected: Bearer <token>")
			return
		}
		apiKeyStr := strings.TrimSpace(parts[1])
		if apiKeyStr == "" {
			log.Debug("Empty API key provided")
			SendUnauthorizedError(c, "Invalid Authorization header: empty token")
			return
		}
		// Add request info to context for audit logging
		requestInfo := &apikey.RequestInfo{
			IPAddress: c.ClientIP(),
			UserAgent: c.GetHeader("User-Agent"),
		}
		ctx := apikey.WithRequestInfo(c.Request.Context(), requestInfo)
		// Validate API key
		key, usr, organization, err := m.apiKeyService.ValidateKey(ctx, apiKeyStr)
		if err != nil {
			// Log the actual error for debugging but return generic message to client
			log.With("error", err).Debug("API key validation failed")
			switch err {
			case apikey.ErrAPIKeyExpired:
				SendUnauthorizedError(c, "API key expired")
			case apikey.ErrInvalidAPIKey:
				SendUnauthorizedError(c, "Invalid API key")
			default:
				SendInternalServerError(c, "Authentication service unavailable")
			}
			return
		}
		// Check if user is active
		if usr.Status != user.StatusActive {
			log.With("user_id", usr.ID, "status", usr.Status).Debug("User account not active")
			SendForbiddenError(c, "User account is not active")
			return
		}
		// Check if organization is active
		if organization.Status != org.StatusActive {
			log.With("org_id", organization.ID, "status", organization.Status).Debug("Organization not active")
			SendForbiddenError(c, "Organization is not active")
			return
		}
		// Store authenticated entities in request context
		// Note: IDs and roles are now accessed via the objects themselves
		ctx = WithOrganization(ctx, organization)
		ctx = WithUser(ctx, usr)
		ctx = WithAPIKey(ctx, key)
		c.Request = c.Request.WithContext(ctx)
		// Log successful authentication
		log.With(
			"user_id", usr.ID,
			"org_id", organization.ID,
			"api_key_id", key.ID,
			"user_role", usr.Role,
		).Debug("Request authenticated successfully")
		c.Next()
	}
}

// GetAPIKey retrieves the authenticated API key from context
func GetAPIKey(c *gin.Context) (*apikey.APIKey, bool) {
	return APIKeyFromContext(c.Request.Context())
}

// GetUser retrieves the authenticated user from context
func GetUser(c *gin.Context) (*user.User, bool) {
	return UserFromContext(c.Request.Context())
}

// GetOrganization retrieves the authenticated organization from context
func GetOrganization(c *gin.Context) (*org.Organization, bool) {
	return OrganizationFromContext(c.Request.Context())
}

// GetUserRole retrieves the authenticated user's role from context
func GetUserRole(c *gin.Context) (user.Role, bool) {
	return UserRoleFromContext(c.Request.Context())
}

// GetOrgID retrieves the authenticated organization ID from context
func GetOrgID(c *gin.Context) (core.ID, bool) {
	return OrgIDFromContext(c.Request.Context())
}

// GetUserID retrieves the authenticated user ID from context
func GetUserID(c *gin.Context) (core.ID, bool) {
	return UserIDFromContext(c.Request.Context())
}

// RequireRole creates a middleware that checks if the authenticated user has one of the required roles
func RequireRole(roles ...user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := GetUserRole(c)
		if !exists {
			SendUnauthorizedError(c, "Authentication required")
			return
		}
		// Check if user has one of the required roles
		hasRole := false
		for _, requiredRole := range roles {
			if userRole == requiredRole {
				hasRole = true
				break
			}
		}
		if !hasRole {
			log := logger.FromContext(c.Request.Context())
			log.With("user_role", userRole, "required_roles", roles).Debug("Insufficient permissions")
			SendForbiddenError(c, "Insufficient permissions")
			return
		}
		c.Next()
	}
}

// RequirePermission creates a middleware that checks if the authenticated user has the required permission
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		usr, exists := GetUser(c)
		if !exists {
			SendUnauthorizedError(c, "Authentication required")
			return
		}
		// Check if user's role has the required permission
		if !usr.Role.HasPermission(permission) {
			log := logger.FromContext(c.Request.Context())
			log.With("user_role", usr.Role, "permission", permission).Debug("Permission denied")
			SendForbiddenError(c, "Insufficient permissions")
			return
		}
		c.Next()
	}
}
