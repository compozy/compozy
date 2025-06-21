package auth_test

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestContextFunctions(t *testing.T) {
	t.Run("Should store and retrieve organization from context", func(t *testing.T) {
		// Create test organization
		org := &org.Organization{
			ID:     core.MustNewID(),
			Name:   "Test Org",
			Status: org.StatusActive,
		}
		// Store in context
		ctx := context.Background()
		ctx = auth.WithOrganization(ctx, org)
		// Retrieve from context
		retrieved, ok := auth.OrganizationFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, org.ID, retrieved.ID)
		assert.Equal(t, org.Name, retrieved.Name)
	})
	t.Run("Should store and retrieve org ID from context", func(t *testing.T) {
		// Create test org ID
		orgID := core.MustNewID()
		// Store in context
		ctx := context.Background()
		ctx = auth.WithOrgID(ctx, orgID)
		// Retrieve from context
		retrieved, ok := auth.OrgIDFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, orgID, retrieved)
	})
	t.Run("Should store and retrieve user from context", func(t *testing.T) {
		// Create test user
		usr := &user.User{
			ID:     core.MustNewID(),
			OrgID:  core.MustNewID(),
			Email:  "test@example.com",
			Role:   user.RoleOrgAdmin,
			Status: user.StatusActive,
		}
		// Store in context
		ctx := context.Background()
		ctx = auth.WithUser(ctx, usr)
		// Retrieve from context
		retrieved, ok := auth.UserFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, usr.ID, retrieved.ID)
		assert.Equal(t, usr.Email, retrieved.Email)
	})
	t.Run("Should store and retrieve user ID from context", func(t *testing.T) {
		// Create test user ID
		userID := core.MustNewID()
		// Store in context
		ctx := context.Background()
		ctx = auth.WithUserID(ctx, userID)
		// Retrieve from context
		retrieved, ok := auth.UserIDFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, userID, retrieved)
	})
	t.Run("Should store and retrieve user role from context", func(t *testing.T) {
		// Create test role
		role := user.RoleOrgManager
		// Store in context
		ctx := context.Background()
		ctx = auth.WithUserRole(ctx, role)
		// Retrieve from context
		retrieved, ok := auth.UserRoleFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, role, retrieved)
	})
	t.Run("Should store and retrieve API key from context", func(t *testing.T) {
		// Create test API key
		key := &apikey.APIKey{
			ID:     core.MustNewID(),
			OrgID:  core.MustNewID(),
			UserID: core.MustNewID(),
			Name:   "Test Key",
			Status: apikey.StatusActive,
		}
		// Store in context
		ctx := context.Background()
		ctx = auth.WithAPIKey(ctx, key)
		// Retrieve from context
		retrieved, ok := auth.APIKeyFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, key.ID, retrieved.ID)
		assert.Equal(t, key.Name, retrieved.Name)
	})
	t.Run("Should panic when required values are missing", func(t *testing.T) {
		ctx := context.Background()
		// Test MustGetOrgID
		assert.Panics(t, func() {
			auth.MustGetOrgID(ctx)
		})
		// Test MustGetUserID
		assert.Panics(t, func() {
			auth.MustGetUserID(ctx)
		})
		// Test MustGetUserRole
		assert.Panics(t, func() {
			auth.MustGetUserRole(ctx)
		})
	})
	t.Run("Should not panic when required values exist", func(t *testing.T) {
		orgID := core.MustNewID()
		userID := core.MustNewID()
		role := user.RoleOrgAdmin
		ctx := context.Background()
		ctx = auth.WithOrgID(ctx, orgID)
		ctx = auth.WithUserID(ctx, userID)
		ctx = auth.WithUserRole(ctx, role)
		// These should not panic
		assert.NotPanics(t, func() {
			gotOrgID := auth.MustGetOrgID(ctx)
			assert.Equal(t, orgID, gotOrgID)
		})
		assert.NotPanics(t, func() {
			gotUserID := auth.MustGetUserID(ctx)
			assert.Equal(t, userID, gotUserID)
		})
		assert.NotPanics(t, func() {
			gotRole := auth.MustGetUserRole(ctx)
			assert.Equal(t, role, gotRole)
		})
	})
}
