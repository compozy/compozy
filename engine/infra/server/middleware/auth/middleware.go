package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/metric"
)

// Manager handles authentication middleware
type Manager struct {
	factory *uc.Factory
	meter   metric.Meter
	config  *config.Config
}

// NewManager creates a new auth middleware manager
func NewManager(factory *uc.Factory, cfg *config.Config) *Manager {
	return &Manager{
		factory: factory,
		config:  cfg,
	}
}

// WithMetrics adds metrics instrumentation to the manager
func (m *Manager) WithMetrics(ctx context.Context, meter metric.Meter) *Manager {
	m.meter = meter

	// Initialize auth metrics
	if meter != nil {
		if err := auth.InitMetrics(meter); err != nil {
			log := logger.FromContext(ctx)
			log.Error("Failed to initialize auth metrics", "error", err)
		}
	}

	return m
}

// Middleware returns the authentication middleware
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Respect runtime config: short‑circuit when auth is disabled
		// Prefer explicitly attached config in context; avoid default fallback in tests
		if c.Request.Context().Value(config.ManagerCtxKey) != nil {
			if cfg := config.FromContext(c.Request.Context()); cfg != nil && !cfg.Server.Auth.Enabled {
				c.Next()
				return
			}
		} else if m.config != nil && !m.config.Server.Auth.Enabled { // fallback for tests
			c.Next()
			return
		}
		start := time.Now()
		ctx := c.Request.Context()
		log := logger.FromContext(ctx)
		authMethod := auth.AuthMethodAPIKey

		// Extract bearer token
		apiKey, err := m.extractBearerToken(c)
		if err != nil {
			var authErr *authError
			recorded := false
			if errors.As(err, &authErr) {
				reason := categorizeHeaderError(authErr)
				auth.RecordAuthAttempt(ctx, auth.AuthOutcomeFailure, reason, authMethod, time.Since(start))
				recorded = true
				if authErr.message == "no authorization header" {
					// Allow endpoints to continue without user context while still tracking failures
					c.Next()
					return
				}
			}
			log.Debug("Authentication failed", "reason", err.Error())
			if !recorded {
				auth.RecordAuthAttempt(ctx, auth.AuthOutcomeFailure, auth.ReasonUnknown, authMethod, time.Since(start))
			}
			m.handleAuthError(c, err)
			return
		}

		validateUC := m.factory.ValidateAPIKey(apiKey)
		user, err := validateUC.Execute(ctx)
		if err != nil {
			log.Debug("API key validation failed")
			reason := categorizeValidationError(err)
			auth.RecordAuthAttempt(ctx, auth.AuthOutcomeFailure, reason, authMethod, time.Since(start))
			m.handleAuthError(c, err)
			return
		}

		auth.RecordAuthAttempt(ctx, auth.AuthOutcomeSuccess, auth.ReasonNone, authMethod, time.Since(start))
		m.setAuthContext(c, apiKey, user)
		c.Next()
	}
}

// extractBearerToken extracts and validates the bearer token
func (m *Manager) extractBearerToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", &authError{message: "no authorization header"}
	}

	// Case-insensitive bearer check and handle extra spaces
	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", &authError{message: "invalid format", public: true}
	}

	apiKey := strings.TrimSpace(parts[1])
	if apiKey == "" {
		return "", &authError{message: "empty token", public: true}
	}

	return apiKey, nil
}

// handleAuthError sends appropriate error response
func (m *Manager) handleAuthError(c *gin.Context, err error) {
	// Generic error message to prevent information leakage
	response := gin.H{
		"error":   "Authentication failed",
		"details": "Invalid or missing credentials",
	}

	// Only provide specific errors for format issues
	if authErr, ok := err.(*authError); ok && authErr.public {
		response["details"] = "Invalid authorization header format"
	}

	c.JSON(401, response)
	c.Abort()
}

func categorizeHeaderError(err *authError) auth.AuthFailureReason {
	if err == nil {
		return auth.ReasonUnknown
	}

	switch err.message {
	case "no authorization header":
		return auth.ReasonMissingAuth
	case "invalid format", "empty token":
		return auth.ReasonInvalidFormat
	default:
		return auth.ReasonUnknown
	}
}

func categorizeValidationError(err error) auth.AuthFailureReason {
	if err == nil {
		return auth.ReasonUnknown
	}

	switch {
	case errors.Is(err, uc.ErrInvalidCredentials):
		return auth.ReasonInvalidCredentials
	case errors.Is(err, uc.ErrTokenExpired):
		return auth.ReasonExpiredToken
	case errors.Is(err, uc.ErrRateLimited):
		return auth.ReasonRateLimited
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "expired") {
		return auth.ReasonExpiredToken
	}
	if strings.Contains(message, "rate limit") {
		return auth.ReasonRateLimited
	}

	return auth.ReasonUnknown
}

// setAuthContext sets authentication information in context
func (m *Manager) setAuthContext(c *gin.Context, apiKey string, user *model.User) {
	// Set in Gin context for rate limiting
	c.Set(auth.ContextKeyAPIKey, apiKey)
	c.Set(auth.ContextKeyUserID, user.ID.String())
	c.Set(auth.ContextKeyUserRole, string(user.Role))

	// Inject into request context
	ctx := userctx.WithUser(c.Request.Context(), user)
	c.Request = c.Request.WithContext(ctx)

	log := logger.FromContext(ctx)
	log.Debug("Authentication successful", "user_id", user.ID)
}

// authError represents an authentication error
type authError struct {
	message string
	public  bool // whether error details can be shown publicly
}

func (e *authError) Error() string {
	return e.message
}

// RequireAuth returns middleware that requires authentication
func (m *Manager) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// No-op when auth is disabled
		if c.Request.Context().Value(config.ManagerCtxKey) != nil {
			if cfg := config.FromContext(c.Request.Context()); cfg != nil && !cfg.Server.Auth.Enabled {
				c.Next()
				return
			}
		} else if m.config != nil && !m.config.Server.Auth.Enabled { // fallback for tests
			c.Next()
			return
		}
		if _, ok := userctx.UserFromContext(c.Request.Context()); !ok {
			c.JSON(401, gin.H{"error": "Authentication required", "details": "This endpoint requires a valid API key"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAdmin returns middleware that requires admin role
func (m *Manager) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// No-op when auth is disabled
		if c.Request.Context().Value(config.ManagerCtxKey) != nil {
			if cfg := config.FromContext(c.Request.Context()); cfg != nil && !cfg.Server.Auth.Enabled {
				c.Next()
				return
			}
		} else if m.config != nil && !m.config.Server.Auth.Enabled { // fallback for tests
			c.Next()
			return
		}
		user, ok := userctx.UserFromContext(c.Request.Context())
		if !ok || user.Role != model.RoleAdmin {
			c.JSON(403, gin.H{"error": "Admin access required", "details": "This endpoint requires admin privileges"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminOnly is an alias for RequireAdmin to match tech spec naming
func (m *Manager) AdminOnly() gin.HandlerFunc {
	return m.RequireAdmin()
}
