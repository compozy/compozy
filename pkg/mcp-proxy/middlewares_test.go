package mcpproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestAuthMiddleware_TokenValidation has been removed as authentication
// functionality has been removed from the MCP proxy server.
// The proxy server no longer provides or enforces authentication mechanisms.

// Admin IP allowlist middleware tests were removed along with the feature.

func TestMiddlewareWrapper_PanicRecovery(t *testing.T) {
	t.Run("Should recover from handler panics and return 500 error", func(t *testing.T) {
		ginmode.EnsureGinTestMode()

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
		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "Internal server error", resp["error"])
		assert.Equal(t, "An unexpected error occurred", resp["details"])
	})

	t.Run("Should handle nil handler gracefully with error response", func(t *testing.T) {
		ginmode.EnsureGinTestMode()

		wrappedHandler := wrapWithGinMiddlewares(nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		c.Request = req

		wrappedHandler(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "Handler not initialized", resp["error"])
		assert.Equal(t, "Handler not initialized", resp["details"])
	})

	t.Run("Should execute middlewares in correct order with proper chaining", func(t *testing.T) {
		ginmode.EnsureGinTestMode()

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
