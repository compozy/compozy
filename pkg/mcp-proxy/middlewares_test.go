package mcpproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCombineAuthTokens(t *testing.T) {
	t.Run("Should return client tokens when global tokens are empty", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: []string{},
		}
		result := combineAuthTokens(proxyHandlers.globalAuthTokens, []string{"token1", "token2"})
		assert.Equal(t, []string{"token1", "token2"}, result)
	})

	t.Run("Should return global tokens when client tokens are empty", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: []string{"global1", "global2"},
		}
		result := combineAuthTokens(proxyHandlers.globalAuthTokens, []string{})
		assert.Equal(t, []string{"global1", "global2"}, result)
	})

	t.Run("Should return empty slice when both are empty", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: []string{},
		}
		result := combineAuthTokens(proxyHandlers.globalAuthTokens, []string{})
		assert.Empty(t, result)
	})

	t.Run("Should combine tokens and remove duplicates", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: []string{"token1", "token2"},
		}
		result := combineAuthTokens(proxyHandlers.globalAuthTokens, []string{"token2", "token3"})
		assert.Equal(t, []string{"token1", "token2", "token3"}, result)
	})

	t.Run("Should skip empty tokens", func(t *testing.T) {
		proxyHandlers := &ProxyHandlers{
			globalAuthTokens: []string{"token1", "", "token2"},
		}
		result := combineAuthTokens(proxyHandlers.globalAuthTokens, []string{"", "token3"})
		assert.Equal(t, []string{"token1", "token2", "token3"}, result)
	})
}

func TestNewAuthMiddleware(t *testing.T) {
	t.Run("Should accept valid Bearer token", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		middleware := newAuthMiddleware([]string{"valid-token"})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Bearer valid-token")
		c.Request = req

		middleware(c)

		assert.False(t, c.IsAborted())
	})

	t.Run("Should accept case-insensitive Bearer token", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		middleware := newAuthMiddleware([]string{"valid-token"})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "bearer valid-token")
		c.Request = req

		middleware(c)

		assert.False(t, c.IsAborted())
	})

	t.Run("Should accept mixed case Bearer token", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		middleware := newAuthMiddleware([]string{"valid-token"})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "BeArEr valid-token")
		c.Request = req

		middleware(c)

		assert.False(t, c.IsAborted())
	})

	t.Run("Should reject invalid token", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		middleware := newAuthMiddleware([]string{"valid-token"})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Bearer invalid-token")
		c.Request = req

		middleware(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should reject missing authorization header", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		middleware := newAuthMiddleware([]string{"valid-token"})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		c.Request = req

		middleware(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should reject non-Bearer authorization", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		middleware := newAuthMiddleware([]string{"valid-token"})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		c.Request = req

		middleware(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Should skip empty tokens during initialization", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		middleware := newAuthMiddleware([]string{"valid-token", "", "another-token"})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Bearer another-token")
		c.Request = req

		middleware(c)

		assert.False(t, c.IsAborted())
	})
}
