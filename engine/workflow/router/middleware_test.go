package wfrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAuthClaims_HasPermission(t *testing.T) {
	t.Run("Should return true when permission exists", func(t *testing.T) {
		claims := &AuthClaims{
			Permissions: []string{"events:write", "events:read"},
		}
		assert.True(t, claims.HasPermission("events:write"))
	})

	t.Run("Should return false when permission does not exist", func(t *testing.T) {
		claims := &AuthClaims{
			Permissions: []string{"events:read"},
		}
		assert.False(t, claims.HasPermission("events:write"))
	})

	t.Run("Should return false when permissions are empty", func(t *testing.T) {
		claims := &AuthClaims{
			Permissions: []string{},
		}
		assert.False(t, claims.HasPermission("events:write"))
	})
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should return 401 when Authorization header is missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		AuthMiddleware()(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should return 401 when Authorization header format is invalid", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		c.Request.Header.Set("Authorization", "Invalid token")
		AuthMiddleware()(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should return 401 when Bearer token is empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer ")
		AuthMiddleware()(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should return 401 when Bearer token is invalid", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer invalid-token")
		AuthMiddleware()(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should process request when valid Bearer token is provided", func(t *testing.T) {
		// Set environment variable for test
		t.Setenv("COMPOZY_API_TOKEN", "test-token")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer test-token")

		middleware := AuthMiddleware()
		middleware(c)

		assert.False(t, c.IsAborted())
		assert.NotNil(t, c.Request.Context().Value(authClaimsKey))
	})

	t.Run("Should process request with dev-token when no env var is set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		c.Request.Header.Set("Authorization", "Bearer dev-token")

		middleware := AuthMiddleware()
		middleware(c)

		assert.False(t, c.IsAborted())
		assert.NotNil(t, c.Request.Context().Value(authClaimsKey))
	})
}

func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should return 500 when auth context is missing", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		RequirePermission("events:write")(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Should return 403 when permission is insufficient", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		claims := &AuthClaims{
			Permissions: []string{"events:read"},
		}
		ctx := context.WithValue(c.Request.Context(), authClaimsKey, claims)
		c.Request = c.Request.WithContext(ctx)
		RequirePermission("events:write")(c)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("Should process request when permission is sufficient", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/events", http.NoBody)
		claims := &AuthClaims{
			Permissions: []string{"events:write"},
		}
		ctx := context.WithValue(c.Request.Context(), authClaimsKey, claims)
		c.Request = c.Request.WithContext(ctx)

		middleware := RequirePermission("events:write")
		middleware(c)

		assert.False(t, c.IsAborted())
	})
}
