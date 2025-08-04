package mcpproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_TokenValidation(t *testing.T) {
	t.Run("Should accept valid Bearer token and allow request through", func(t *testing.T) {
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

	t.Run("Should accept case-insensitive Bearer token header", func(t *testing.T) {
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

	t.Run("Should accept mixed case Bearer prefix in authorization header", func(t *testing.T) {
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

	t.Run("Should reject request with invalid auth token", func(t *testing.T) {
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

	t.Run("Should reject request without authorization header", func(t *testing.T) {
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

	t.Run("Should reject non-Bearer authorization schemes", func(t *testing.T) {
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

	t.Run("Should filter out empty tokens and validate non-empty ones", func(t *testing.T) {
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

func TestMiddlewareWrapper_PanicRecovery(t *testing.T) {
	t.Run("Should recover from handler panics and return 500 error", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		// Create a handler that panics
		panicHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("test panic")
		})

		// Create recovery middleware
		recoverMW := recoverMiddleware("test-client")

		// Wrap handler with recovery middleware
		wrappedHandler := wrapWithGinMiddlewares(panicHandler, recoverMW)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		c.Request = req

		// This should not panic and should return 500
		wrappedHandler(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Internal server error")
	})

	t.Run("Should handle nil handler gracefully with error response", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		wrappedHandler := wrapWithGinMiddlewares(nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		c.Request = req

		wrappedHandler(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Handler not initialized")
	})

	t.Run("Should execute middlewares in correct order with proper chaining", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		var executionOrder []string

		middleware1 := func(c *gin.Context) {
			executionOrder = append(executionOrder, "middleware1-start")
			c.Next()
			executionOrder = append(executionOrder, "middleware1-end")
		}

		middleware2 := func(c *gin.Context) {
			executionOrder = append(executionOrder, "middleware2-start")
			c.Next()
			executionOrder = append(executionOrder, "middleware2-end")
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			executionOrder = append(executionOrder, "handler")
			w.WriteHeader(http.StatusOK)
		})

		wrappedHandler := wrapWithGinMiddlewares(handler, middleware1, middleware2)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		c.Request = req

		wrappedHandler(c)

		assert.Equal(t, http.StatusOK, w.Code)
		// Middleware should execute in order: 1-start, 2-start, handler, 2-end, 1-end
		expected := []string{"middleware1-start", "middleware2-start", "handler", "middleware2-end", "middleware1-end"}
		assert.Equal(t, expected, executionOrder)
	})
}
