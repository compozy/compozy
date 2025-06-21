package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_SimpleValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should reject missing authorization header", func(t *testing.T) {
		// Create a dummy service (won't be called for this test)
		middleware := &Middleware{
			apiKeyService: nil, // Won't be used for this test case
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.True(t, c.IsAborted())
		assert.Contains(t, w.Body.String(), "Missing Authorization header")
	})
	t.Run("Should reject invalid authorization format", func(t *testing.T) {
		middleware := &Middleware{
			apiKeyService: nil, // Won't be used for this test case
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "InvalidFormat")
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid Authorization header format")
	})
	t.Run("Should reject non-bearer token", func(t *testing.T) {
		middleware := &Middleware{
			apiKeyService: nil, // Won't be used for this test case
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid Authorization header format")
	})
	t.Run("Should reject empty token", func(t *testing.T) {
		middleware := &Middleware{
			apiKeyService: nil, // Won't be used for this test case
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer ")
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "empty token")
	})
	t.Run("Should reject whitespace-only token", func(t *testing.T) {
		middleware := &Middleware{
			apiKeyService: nil, // Won't be used for this test case
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer   \t\n  ")
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "empty token")
	})
}

func TestRequireRole_Simple(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should reject when no user in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		middleware := RequireRole(user.RoleOrgAdmin)
		middleware(c)
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Authentication required")
	})
	t.Run("Should allow user with required role", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Add user to context
		ctx := c.Request.Context()
		testUser := &user.User{
			ID:     core.MustNewID(),
			Role:   user.RoleOrgAdmin,
			Status: user.StatusActive,
		}
		ctx = WithUser(ctx, testUser)
		c.Request = c.Request.WithContext(ctx)
		// Execute middleware
		middleware := RequireRole(user.RoleOrgAdmin)
		middleware(c)
		// Assert - should pass through
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("Should reject user without required role", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Add user to context
		ctx := c.Request.Context()
		testUser := &user.User{
			ID:     core.MustNewID(),
			Role:   user.RoleOrgCustomer, // Does not have admin role
			Status: user.StatusActive,
		}
		ctx = WithUser(ctx, testUser)
		c.Request = c.Request.WithContext(ctx)
		// Execute middleware
		middleware := RequireRole(user.RoleOrgAdmin)
		middleware(c)
		// Assert
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Insufficient permissions")
	})
}

func TestRequirePermission_Simple(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should reject when no user in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Execute middleware
		middleware := RequirePermission("workflow:create")
		middleware(c)
		// Assert
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Authentication required")
	})
	t.Run("Should allow user with required permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Add user to context
		ctx := c.Request.Context()
		testUser := &user.User{
			ID:     core.MustNewID(),
			Role:   user.RoleOrgAdmin, // Has workflow:create permission
			Status: user.StatusActive,
		}
		ctx = WithUser(ctx, testUser)
		c.Request = c.Request.WithContext(ctx)
		// Execute middleware
		middleware := RequirePermission("workflow:create")
		middleware(c)
		// Assert - should pass through
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("Should reject user without required permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Add user to context
		ctx := c.Request.Context()
		testUser := &user.User{
			ID:     core.MustNewID(),
			Role:   user.RoleOrgCustomer, // Does not have user:manage permission
			Status: user.StatusActive,
		}
		ctx = WithUser(ctx, testUser)
		c.Request = c.Request.WithContext(ctx)
		// Execute middleware
		middleware := RequirePermission("user:manage")
		middleware(c)
		// Assert
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Insufficient permissions")
	})
	t.Run("Should allow system admin any permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Add user to context
		ctx := c.Request.Context()
		testUser := &user.User{
			ID:     core.MustNewID(),
			Role:   user.RoleSystemAdmin,
			Status: user.StatusActive,
		}
		ctx = WithUser(ctx, testUser)
		c.Request = c.Request.WithContext(ctx)
		// Execute middleware
		middleware := RequirePermission("any:permission")
		middleware(c)
		// Assert - should pass through
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGetAuthenticationHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should retrieve authentication data from context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Create test data
		testUser := &user.User{
			ID:     core.MustNewID(),
			OrgID:  core.MustNewID(),
			Email:  "test@example.com",
			Role:   user.RoleOrgAdmin,
			Status: user.StatusActive,
		}
		testOrg := &org.Organization{
			ID:     testUser.OrgID,
			Name:   "Test Org",
			Status: org.StatusActive,
		}
		testKey := &apikey.APIKey{
			ID:     core.MustNewID(),
			UserID: testUser.ID,
			OrgID:  testUser.OrgID,
		}
		// Add to context
		ctx := c.Request.Context()
		ctx = WithOrganization(ctx, testOrg)
		ctx = WithUser(ctx, testUser)
		ctx = WithAPIKey(ctx, testKey)
		c.Request = c.Request.WithContext(ctx)
		// Test retrieval functions
		retrievedOrg, ok := GetOrganization(c)
		require.True(t, ok)
		assert.Equal(t, testOrg.ID, retrievedOrg.ID)
		assert.Equal(t, testOrg.Name, retrievedOrg.Name)
		retrievedUser, ok := GetUser(c)
		require.True(t, ok)
		assert.Equal(t, testUser.ID, retrievedUser.ID)
		assert.Equal(t, testUser.Email, retrievedUser.Email)
		assert.Equal(t, testUser.Role, retrievedUser.Role)
		retrievedKey, ok := GetAPIKey(c)
		require.True(t, ok)
		assert.Equal(t, testKey.ID, retrievedKey.ID)
		// Test ID accessors work correctly (accessing via objects)
		orgID, ok := GetOrgID(c)
		require.True(t, ok)
		assert.Equal(t, testOrg.ID, orgID)
		userID, ok := GetUserID(c)
		require.True(t, ok)
		assert.Equal(t, testUser.ID, userID)
		role, ok := GetUserRole(c)
		require.True(t, ok)
		assert.Equal(t, testUser.Role, role)
	})
	t.Run("Should return false when no authentication data in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		// Test retrieval functions
		_, ok := GetOrganization(c)
		assert.False(t, ok)
		_, ok = GetUser(c)
		assert.False(t, ok)
		_, ok = GetAPIKey(c)
		assert.False(t, ok)
		_, ok = GetOrgID(c)
		assert.False(t, ok)
		_, ok = GetUserID(c)
		assert.False(t, ok)
		_, ok = GetUserRole(c)
		assert.False(t, ok)
	})
}
