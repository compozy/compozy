package auth

import (
	"context"

	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/gin-gonic/gin"
)

// OrgContextMiddleware is deprecated as context injection is now handled by AuthMiddleware
// Deprecated: Use AuthMiddleware which now handles all context injection
type OrgContextMiddleware struct{}

// NewOrgContextMiddleware creates a new organization context middleware
// Deprecated: Use AuthMiddleware which now handles all context injection
func NewOrgContextMiddleware() *OrgContextMiddleware {
	return &OrgContextMiddleware{}
}

// InjectOrgContext is deprecated as context injection is now handled by AuthMiddleware
// Deprecated: Use AuthMiddleware which now handles all context injection
func (m *OrgContextMiddleware) InjectOrgContext() gin.HandlerFunc {
	// This is now a no-op middleware for backward compatibility
	return func(c *gin.Context) {
		c.Next()
	}
}

// Context injection functions

// WithOrganization adds the organization to the context
func WithOrganization(ctx context.Context, organization *org.Organization) context.Context {
	return context.WithValue(ctx, contextKeyOrg, organization)
}

// WithOrgID adds the organization ID to the context
func WithOrgID(ctx context.Context, orgID core.ID) context.Context {
	return context.WithValue(ctx, contextKeyOrgID, orgID)
}

// WithUser adds the user to the context
func WithUser(ctx context.Context, usr *user.User) context.Context {
	return context.WithValue(ctx, contextKeyUser, usr)
}

// WithUserID adds the user ID to the context
func WithUserID(ctx context.Context, userID core.ID) context.Context {
	return context.WithValue(ctx, contextKeyUserID, userID)
}

// WithUserRole adds the user role to the context
func WithUserRole(ctx context.Context, role user.Role) context.Context {
	return context.WithValue(ctx, contextKeyUserRole, role)
}

// WithAPIKey adds the API key to the context
func WithAPIKey(ctx context.Context, key *apikey.APIKey) context.Context {
	return context.WithValue(ctx, contextKeyAPIKey, key)
}

// Context retrieval functions

// OrganizationFromContext retrieves the organization from context
func OrganizationFromContext(ctx context.Context) (*org.Organization, bool) {
	organization, ok := ctx.Value(contextKeyOrg).(*org.Organization)
	return organization, ok
}

// OrgIDFromContext retrieves the organization ID from context
func OrgIDFromContext(ctx context.Context) (core.ID, bool) {
	orgID, ok := ctx.Value(contextKeyOrgID).(core.ID)
	return orgID, ok
}

// UserFromContext retrieves the user from context
func UserFromContext(ctx context.Context) (*user.User, bool) {
	usr, ok := ctx.Value(contextKeyUser).(*user.User)
	return usr, ok
}

// UserIDFromContext retrieves the user ID from context
func UserIDFromContext(ctx context.Context) (core.ID, bool) {
	userID, ok := ctx.Value(contextKeyUserID).(core.ID)
	return userID, ok
}

// UserRoleFromContext retrieves the user role from context
func UserRoleFromContext(ctx context.Context) (user.Role, bool) {
	role, ok := ctx.Value(contextKeyUserRole).(user.Role)
	return role, ok
}

// APIKeyFromContext retrieves the API key from context
func APIKeyFromContext(ctx context.Context) (*apikey.APIKey, bool) {
	key, ok := ctx.Value(contextKeyAPIKey).(*apikey.APIKey)
	return key, ok
}

// MustGetOrgID retrieves the organization ID from context or panics
func MustGetOrgID(ctx context.Context) core.ID {
	orgID, ok := OrgIDFromContext(ctx)
	if !ok {
		panic("organization ID not found in context")
	}
	return orgID
}

// MustGetUserID retrieves the user ID from context or panics
func MustGetUserID(ctx context.Context) core.ID {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		panic("user ID not found in context")
	}
	return userID
}

// MustGetUserRole retrieves the user role from context or panics
func MustGetUserRole(ctx context.Context) user.Role {
	role, ok := UserRoleFromContext(ctx)
	if !ok {
		panic("user role not found in context")
	}
	return role
}
