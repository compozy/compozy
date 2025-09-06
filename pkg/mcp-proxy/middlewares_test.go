package mcpproxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestAuthMiddleware_TokenValidation has been removed as authentication
// functionality has been removed from the MCP proxy server.
// The proxy server no longer provides or enforces authentication mechanisms.

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
