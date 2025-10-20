package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	authmetrics "github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/infra/server/router"
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
	if meter != nil {
		if err := authmetrics.InitMetrics(meter); err != nil {
			log := logger.FromContext(ctx)
			log.Error("Failed to initialize auth metrics", "error", err)
		}
	}
	return m
}

// Middleware returns the authentication middleware
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.shouldSkipAuth(c) {
			c.Next()
			return
		}
		start := time.Now()
		ctx := c.Request.Context()
		authMethod := authmetrics.AuthMethodAPIKey
		apiKey, err := m.extractBearerToken(c)
		if err != nil {
			m.handleTokenExtractionFailure(ctx, c, err, start, authMethod)
			return
		}
		if !m.validateAPIKeyAndSetContext(ctx, c, apiKey, start, authMethod) {
			return
		}
		c.Next()
	}
}

// shouldSkipAuth determines if authentication is disabled at runtime.
func (m *Manager) shouldSkipAuth(c *gin.Context) bool {
	if c.Request.Context().Value(config.ManagerCtxKey) != nil {
		if cfg := config.FromContext(c.Request.Context()); cfg != nil && !cfg.Server.Auth.Enabled {
			return true
		}
	} else if m.config != nil && !m.config.Server.Auth.Enabled {
		return true
	}
	return false
}

// handleTokenExtractionFailure logs, records metrics, and responds when token extraction fails.
func (m *Manager) handleTokenExtractionFailure(
	ctx context.Context,
	c *gin.Context,
	err error,
	start time.Time,
	authMethod authmetrics.AuthMethod,
) {
	log := logger.FromContext(ctx)
	var authErr *authError
	recorded := false
	if errors.As(err, &authErr) {
		reason := categorizeHeaderError(authErr)
		authmetrics.RecordAuthAttempt(
			ctx,
			authmetrics.AuthOutcomeFailure,
			reason,
			authMethod,
			time.Since(start),
		)
		recorded = true
		if authErr.message == "no authorization header" {
			c.Next()
			return
		}
	}
	log.Debug("Authentication failed", "reason", err.Error())
	if !recorded {
		authmetrics.RecordAuthAttempt(
			ctx,
			authmetrics.AuthOutcomeFailure,
			authmetrics.ReasonUnknown,
			authMethod,
			time.Since(start),
		)
	}
	m.handleAuthError(c, err)
}

// validateAPIKeyAndSetContext validates the API key and configures user context.
func (m *Manager) validateAPIKeyAndSetContext(
	ctx context.Context,
	c *gin.Context,
	apiKey string,
	start time.Time,
	authMethod authmetrics.AuthMethod,
) bool {
	user, err := m.factory.ValidateAPIKey(apiKey).Execute(ctx)
	if err != nil {
		logger.FromContext(ctx).Debug("API key validation failed")
		reason := categorizeValidationError(err)
		authmetrics.RecordAuthAttempt(
			ctx,
			authmetrics.AuthOutcomeFailure,
			reason,
			authMethod,
			time.Since(start),
		)
		m.handleAuthError(c, err)
		return false
	}
	authmetrics.RecordAuthAttempt(
		ctx,
		authmetrics.AuthOutcomeSuccess,
		authmetrics.ReasonNone,
		authMethod,
		time.Since(start),
	)
	m.setAuthContext(c, apiKey, user)
	return true
}

// extractBearerToken extracts and validates the bearer token
func (m *Manager) extractBearerToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", &authError{message: "no authorization header"}
	}
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
	// WARNING: Return a generic auth failure to avoid disclosing credential validation details.
	reason := "Authentication failed"
	details := "Invalid or missing credentials"
	status := http.StatusUnauthorized
	if authErr, ok := err.(*authError); ok && authErr.public {
		details = "Invalid authorization header format"
		status = http.StatusBadRequest
	}
	switch {
	case errors.Is(err, uc.ErrRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, uc.ErrInvalidCredentials), errors.Is(err, uc.ErrTokenExpired):
		status = http.StatusUnauthorized
	}
	if status == http.StatusUnauthorized {
		c.Header("WWW-Authenticate", `Bearer realm="compozy", charset="UTF-8"`)
	}
	router.RespondWithError(c, status, router.NewRequestError(status, reason, errors.New(details)))
	c.Abort()
}

func categorizeHeaderError(err *authError) authmetrics.AuthFailureReason {
	if err == nil {
		return authmetrics.ReasonUnknown
	}
	switch err.message {
	case "no authorization header":
		return authmetrics.ReasonMissingAuth
	case "invalid format", "empty token":
		return authmetrics.ReasonInvalidFormat
	default:
		return authmetrics.ReasonUnknown
	}
}

func categorizeValidationError(err error) authmetrics.AuthFailureReason {
	if err == nil {
		return authmetrics.ReasonUnknown
	}
	switch {
	case errors.Is(err, uc.ErrInvalidCredentials):
		return authmetrics.ReasonInvalidCredentials
	case errors.Is(err, uc.ErrTokenExpired):
		return authmetrics.ReasonExpiredToken
	case errors.Is(err, uc.ErrRateLimited):
		return authmetrics.ReasonRateLimited
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "expired") {
		return authmetrics.ReasonExpiredToken
	}
	if strings.Contains(message, "rate limit") {
		return authmetrics.ReasonRateLimited
	}
	return authmetrics.ReasonUnknown
}

// setAuthContext sets authentication information in context
func (m *Manager) setAuthContext(c *gin.Context, apiKey string, user *model.User) {
	c.Set(authmetrics.ContextKeyAPIKey, apiKey)
	c.Set(authmetrics.ContextKeyUserID, user.ID.String())
	c.Set(authmetrics.ContextKeyUserRole, string(user.Role))
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
			router.RespondWithError(c, http.StatusUnauthorized,
				router.NewRequestError(http.StatusUnauthorized, "Authentication required",
					fmt.Errorf("this endpoint requires a valid API key")))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAdmin returns middleware that requires admin role
func (m *Manager) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
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
			router.RespondWithError(c, http.StatusForbidden,
				router.NewRequestError(http.StatusForbidden, "Admin access required",
					fmt.Errorf("this endpoint requires admin privileges")))
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
