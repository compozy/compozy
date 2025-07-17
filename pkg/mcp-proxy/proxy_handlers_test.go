package mcpproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyHandlers(t *testing.T) {
	// Set gin to test mode
	gin.SetMode(gin.TestMode)
	// Create dependencies
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()

	// Create proxy handlers
	proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:8081", nil)

	// Create a router with proxy endpoints
	router := gin.New()
	router.Any("/:name/sse", proxyHandlers.SSEProxyHandler)
	router.Any("/:name/sse/*path", proxyHandlers.SSEProxyHandler)
	router.Any("/:name/stream", proxyHandlers.StreamableHTTPProxyHandler)
	router.Any("/:name/stream/*path", proxyHandlers.StreamableHTTPProxyHandler)

	t.Run("SSE Proxy Handler - MCP Not Found", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), "GET", "/nonexistent/sse", http.NoBody)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Streamable HTTP Proxy Handler - MCP Not Found", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), "POST", "/nonexistent/stream", http.NoBody)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Register and Access MCP Proxy", func(t *testing.T) {
		// Create a mock MCP definition
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP for proxy",
			Transport:   TransportStdio,
			Command:     "echo",
			Args:        []string{"hello"},
		}

		// Add to storage first
		err := storage.SaveMCP(context.Background(), &mcpDef)
		require.NoError(t, err)

		// Note: We can't fully test the proxy registration without a real MCP client
		// The GetClient call will fail with our mock, which is expected behavior
		// This test just ensures the endpoint routing works

		req, err := http.NewRequestWithContext(context.Background(), "GET", "/test-mcp/sse", http.NoBody)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should get 404 because proxy not registered (mock client fails)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Direct Proxy Server Access - Success Path", func(t *testing.T) {
		// Test successful access when proxy server is already registered
		mockClientManager := NewMockClientManagerWithClient()
		proxyHandlers := NewProxyHandlers(storage, mockClientManager, "http://localhost:8081", nil)

		// Create a new router for this test
		successRouter := gin.New()
		successRouter.Any("/:name/sse", proxyHandlers.SSEProxyHandler)
		successRouter.Any("/:name/stream", proxyHandlers.StreamableHTTPProxyHandler)

		// Manually add a mock proxy server to simulate successful registration
		// This tests the routing logic without the complex MCP initialization
		mockDef := &MCPDefinition{
			Name:        "registered-mcp",
			Description: "Test MCP",
			Transport:   TransportStdio,
			Command:     "echo",
			LogEnabled:  false,
			AuthTokens:  nil,
		}
		mockProxyServer := &ProxyServer{
			mcpServer: nil, // Can be nil for this routing test
			sseServer: nil, // Will be checked but routing will work
			client:    nil,
			def:       mockDef, // Provide a proper definition
		}

		proxyHandlers.serversMutex.Lock()
		proxyHandlers.servers["registered-mcp"] = mockProxyServer
		proxyHandlers.serversMutex.Unlock()

		// Test SSE endpoint access
		req, err := http.NewRequestWithContext(context.Background(), "GET", "/registered-mcp/sse", http.NoBody)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		successRouter.ServeHTTP(w, req)

		// Should not get 404 since proxy is registered
		// May get other errors due to nil server components, but routing works
		assert.NotEqual(t, http.StatusNotFound, w.Code)

		// Verify server can be retrieved
		server := proxyHandlers.GetProxyServer("registered-mcp")
		assert.NotNil(t, server)

		// Test cleanup
		err = proxyHandlers.UnregisterMCPProxy(t.Context(), "registered-mcp")
		assert.NoError(t, err)

		// After unregistration, should get 404 again
		w2 := httptest.NewRecorder()
		successRouter.ServeHTTP(w2, req)
		assert.Equal(t, http.StatusNotFound, w2.Code)
	})
}

func TestProxyServerManagement(t *testing.T) {
	// Create dependencies
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()
	// Create proxy handlers
	proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:8081", nil)

	t.Run("Unregister Nonexistent Proxy", func(t *testing.T) {
		err := proxyHandlers.UnregisterMCPProxy(t.Context(), "nonexistent")
		assert.NoError(t, err) // Should not error, just log warning
	})

	t.Run("Get Proxy Server", func(t *testing.T) {
		server := proxyHandlers.GetProxyServer("nonexistent")
		assert.Nil(t, server)
	})
}
