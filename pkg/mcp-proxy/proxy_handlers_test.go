package mcpproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyHandlers_SSEProxy(t *testing.T) {
	ginmode.EnsureGinTestMode()
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()

	proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:6001")

	router := gin.New()
	router.Any("/mcp-proxy/:name/sse", proxyHandlers.SSEProxyHandler)
	router.Any("/mcp-proxy/:name/sse/*path", proxyHandlers.SSEProxyHandler)
	router.Any("/mcp-proxy/:name/stream", proxyHandlers.StreamableHTTPProxyHandler)
	router.Any("/mcp-proxy/:name/stream/*path", proxyHandlers.StreamableHTTPProxyHandler)

	t.Run("Should return 404 for non-existent MCP server", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), "GET", "/mcp-proxy/nonexistent/sse", http.NoBody)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		// Cover wildcard SSE route path as well
		req2, err := http.NewRequestWithContext(
			context.Background(),
			"GET",
			"/mcp-proxy/nonexistent/sse/anything",
			http.NoBody,
		)
		require.NoError(t, err)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusNotFound, w2.Code)
	})
}

func TestProxyHandlers_StreamableHTTPProxy(t *testing.T) {
	ginmode.EnsureGinTestMode()
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()

	proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:6001")

	router := gin.New()
	router.Any("/mcp-proxy/:name/sse", proxyHandlers.SSEProxyHandler)
	router.Any("/mcp-proxy/:name/sse/*path", proxyHandlers.SSEProxyHandler)
	router.Any("/mcp-proxy/:name/stream", proxyHandlers.StreamableHTTPProxyHandler)
	router.Any("/mcp-proxy/:name/stream/*path", proxyHandlers.StreamableHTTPProxyHandler)

	t.Run("Should return 404 for non-existent MCP server in stream endpoint", func(t *testing.T) {
		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			"/mcp-proxy/nonexistent/stream",
			http.NoBody,
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestProxyHandlers_ProxyRegistration(t *testing.T) {
	ginmode.EnsureGinTestMode()
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()

	proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:6001")

	router := gin.New()
	router.Any("/mcp-proxy/:name/sse", proxyHandlers.SSEProxyHandler)
	router.Any("/mcp-proxy/:name/sse/*path", proxyHandlers.SSEProxyHandler)
	router.Any("/mcp-proxy/:name/stream", proxyHandlers.StreamableHTTPProxyHandler)
	router.Any("/mcp-proxy/:name/stream/*path", proxyHandlers.StreamableHTTPProxyHandler)

	t.Run("Should route to proxy endpoint even when client registration fails", func(t *testing.T) {
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP for proxy",
			Transport:   TransportStdio,
			Command:     "echo",
			Args:        []string{"hello"},
		}

		err := storage.SaveMCP(context.Background(), &mcpDef)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(context.Background(), "GET", "/mcp-proxy/test-mcp/sse", http.NoBody)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Should handle registered proxy server access correctly", func(t *testing.T) {
		// Test successful access when proxy server is already registered
		mockClientManager := NewMockClientManagerWithClient()
		proxyHandlers := NewProxyHandlers(storage, mockClientManager, "http://localhost:6001")

		// Create a new router for this test
		successRouter := gin.New()
		successRouter.Any("/mcp-proxy/:name/sse", proxyHandlers.SSEProxyHandler)
		successRouter.Any("/mcp-proxy/:name/sse/*path", proxyHandlers.SSEProxyHandler)
		successRouter.Any("/mcp-proxy/:name/stream", proxyHandlers.StreamableHTTPProxyHandler)
		successRouter.Any("/mcp-proxy/:name/stream/*path", proxyHandlers.StreamableHTTPProxyHandler)

		// Manually add a mock proxy server to simulate successful registration
		// This tests the routing logic without the complex MCP initialization
		mockDef := &MCPDefinition{
			Name:        "registered-mcp",
			Description: "Test MCP",
			Transport:   TransportStdio,
			Command:     "echo",
			LogEnabled:  false,
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
		req, err := http.NewRequestWithContext(
			context.Background(),
			"GET",
			"/mcp-proxy/registered-mcp/sse",
			http.NoBody,
		)
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

func TestProxyHandlers_ServerManagement(t *testing.T) {
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()
	proxyHandlers := NewProxyHandlers(storage, clientManager, "http://localhost:6001")

	t.Run("Should handle unregistering non-existent proxy without error", func(t *testing.T) {
		err := proxyHandlers.UnregisterMCPProxy(t.Context(), "nonexistent")
		assert.NoError(t, err)
	})

	t.Run("Should return nil for non-existent proxy server", func(t *testing.T) {
		server := proxyHandlers.GetProxyServer("nonexistent")
		assert.Nil(t, server)
	})
}
