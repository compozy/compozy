package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAPIKeyService for integration testing
type MockAPIKeyService struct {
	validateKeyFunc func(context.Context, string) (*apikey.APIKey, *user.User, *org.Organization, error)
}

func (m *MockAPIKeyService) ValidateKey(
	ctx context.Context,
	keyStr string,
) (*apikey.APIKey, *user.User, *org.Organization, error) {
	if m.validateKeyFunc != nil {
		return m.validateKeyFunc(ctx, keyStr)
	}
	return nil, nil, nil, apikey.ErrAPIKeyNotFound
}

func TestAuthMiddleware_IntegrationSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should successfully authenticate valid API key", func(t *testing.T) {
		// Create test entities
		testAPIKey := &apikey.APIKey{
			ID:     core.MustNewID(),
			OrgID:  core.MustNewID(),
			UserID: core.MustNewID(),
			Name:   "Test API Key",
			Status: apikey.StatusActive,
		}
		testUser := &user.User{
			ID:     testAPIKey.UserID,
			OrgID:  testAPIKey.OrgID,
			Email:  "test@example.com",
			Role:   user.RoleOrgAdmin,
			Status: user.StatusActive,
		}
		testOrg := &org.Organization{
			ID:     testAPIKey.OrgID,
			Name:   "Test Organization",
			Status: org.StatusActive,
		}
		// Create mock service
		mockService := &MockAPIKeyService{
			validateKeyFunc: func(ctx context.Context, keyStr string) (*apikey.APIKey, *user.User, *org.Organization, error) {
				assert.Equal(t, "valid-api-key-123", keyStr)
				// Verify request info is in context
				requestInfo := apikey.GetRequestInfo(ctx)
				assert.NotNil(t, requestInfo)
				assert.NotEmpty(t, requestInfo.IPAddress)
				return testAPIKey, testUser, testOrg, nil
			},
		}
		// Create middleware
		middleware := auth.NewAuthMiddleware(mockService)
		// Setup test context
		w := httptest.NewRecorder()
		c, engine := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer valid-api-key-123")
		// Add test handler to verify context is set
		engine.GET("/test", middleware.Authenticate(), func(c *gin.Context) {
			// Verify entities are in context
			retrievedAPIKey, ok := auth.APIKeyFromContext(c.Request.Context())
			require.True(t, ok)
			assert.Equal(t, testAPIKey.ID, retrievedAPIKey.ID)
			retrievedUser, ok := auth.UserFromContext(c.Request.Context())
			require.True(t, ok)
			assert.Equal(t, testUser.ID, retrievedUser.ID)
			assert.Equal(t, testUser.Role, retrievedUser.Role)
			retrievedOrg, ok := auth.OrganizationFromContext(c.Request.Context())
			require.True(t, ok)
			assert.Equal(t, testOrg.ID, retrievedOrg.ID)
			// Verify helper functions work
			orgID, ok := auth.OrgIDFromContext(c.Request.Context())
			require.True(t, ok)
			assert.Equal(t, testOrg.ID, orgID)
			userID, ok := auth.UserIDFromContext(c.Request.Context())
			require.True(t, ok)
			assert.Equal(t, testUser.ID, userID)
			userRole, ok := auth.UserRoleFromContext(c.Request.Context())
			require.True(t, ok)
			assert.Equal(t, testUser.Role, userRole)
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		// Execute request
		engine.ServeHTTP(w, c.Request)
		// Assert success
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})
}

func TestAuthMiddleware_IntegrationFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("Should handle API key service errors", func(t *testing.T) {
		// Create mock service that returns error
		mockService := &MockAPIKeyService{
			validateKeyFunc: func(_ context.Context, _ string) (*apikey.APIKey, *user.User, *org.Organization, error) {
				return nil, nil, nil, apikey.ErrAPIKeyNotFound
			},
		}
		// Create middleware
		middleware := auth.NewAuthMiddleware(mockService)
		// Setup test context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer invalid-key")
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert failure
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.True(t, c.IsAborted())
		assert.Contains(t, w.Body.String(), "Authentication service unavailable")
	})
	t.Run("Should handle suspended user", func(t *testing.T) {
		// Create test entities with suspended user
		testAPIKey := &apikey.APIKey{
			ID:     core.MustNewID(),
			OrgID:  core.MustNewID(),
			UserID: core.MustNewID(),
			Name:   "Test API Key",
			Status: apikey.StatusActive,
		}
		testUser := &user.User{
			ID:     testAPIKey.UserID,
			OrgID:  testAPIKey.OrgID,
			Email:  "test@example.com",
			Role:   user.RoleOrgAdmin,
			Status: user.StatusSuspended, // Suspended user
		}
		testOrg := &org.Organization{
			ID:     testAPIKey.OrgID,
			Name:   "Test Organization",
			Status: org.StatusActive,
		}
		// Create mock service
		mockService := &MockAPIKeyService{
			validateKeyFunc: func(_ context.Context, _ string) (*apikey.APIKey, *user.User, *org.Organization, error) {
				return testAPIKey, testUser, testOrg, nil
			},
		}
		// Create middleware
		middleware := auth.NewAuthMiddleware(mockService)
		// Setup test context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer valid-key")
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert failure
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.True(t, c.IsAborted())
		assert.Contains(t, w.Body.String(), "User account is not active")
	})
	t.Run("Should handle suspended organization", func(t *testing.T) {
		// Create test entities with suspended org
		testAPIKey := &apikey.APIKey{
			ID:     core.MustNewID(),
			OrgID:  core.MustNewID(),
			UserID: core.MustNewID(),
			Name:   "Test API Key",
			Status: apikey.StatusActive,
		}
		testUser := &user.User{
			ID:     testAPIKey.UserID,
			OrgID:  testAPIKey.OrgID,
			Email:  "test@example.com",
			Role:   user.RoleOrgAdmin,
			Status: user.StatusActive,
		}
		testOrg := &org.Organization{
			ID:     testAPIKey.OrgID,
			Name:   "Test Organization",
			Status: org.StatusSuspended, // Suspended org
		}
		// Create mock service
		mockService := &MockAPIKeyService{
			validateKeyFunc: func(_ context.Context, _ string) (*apikey.APIKey, *user.User, *org.Organization, error) {
				return testAPIKey, testUser, testOrg, nil
			},
		}
		// Create middleware
		middleware := auth.NewAuthMiddleware(mockService)
		// Setup test context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer valid-key")
		// Execute middleware
		handler := middleware.Authenticate()
		handler(c)
		// Assert failure
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.True(t, c.IsAborted())
		assert.Contains(t, w.Body.String(), "Organization is not active")
	})
}
