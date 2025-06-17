package mcpproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: This file provides enhanced integration tests for tool functionality.
// Due to Go's type constraints and the current architecture using concrete types,
// full tool execution testing requires architectural changes to use interfaces.
// The tests below focus on API structure and validation.

// TestToolAPIEndpointsIntegration tests the tool API endpoints structure and validation
func TestToolAPIEndpointsIntegration(t *testing.T) {
	t.Run("Should handle tool listing endpoint structure", func(t *testing.T) {
		// Create memory storage
		storage := mcpproxy.NewMemoryStorage()

		// Use standard mock client manager (tool functionality would require architectural changes)
		clientManager := mcpproxy.NewMockClientManager()

		// Create server
		config := &mcpproxy.Config{
			Port:            "8080",
			Host:            "localhost",
			BaseURL:         "http://localhost:8080",
			ShutdownTimeout: 5 * time.Second,
			AdminTokens:     []string{"test-admin-token"},
		}

		server := mcpproxy.NewServer(config, storage, clientManager)

		// Test tool listing endpoint exists and responds with proper structure
		req := httptest.NewRequest(http.MethodGet, "/admin/tools", http.NoBody)
		req.Header.Set("Authorization", "Bearer test-admin-token")
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)

		// Should return 200 even with no tools
		assert.Equal(t, http.StatusOK, w.Code)

		var toolsResponse map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &toolsResponse)
		require.NoError(t, err)

		// Response should have tools array (empty is fine)
		_, ok := toolsResponse["tools"]
		assert.True(t, ok, "Response should contain 'tools' field")
	})

	t.Run("Should handle non-existent MCP in tool call", func(t *testing.T) {
		storage := mcpproxy.NewMemoryStorage()
		clientManager := mcpproxy.NewMockClientManager()

		config := &mcpproxy.Config{
			Port:            "8080",
			Host:            "localhost",
			BaseURL:         "http://localhost:8080",
			ShutdownTimeout: 5 * time.Second,
			AdminTokens:     []string{"test-admin-token"},
		}

		server := mcpproxy.NewServer(config, storage, clientManager)

		// Test calling tool on non-existent MCP should return 404
		toolCall := mcpproxy.CallToolRequest{
			MCPName:   "non-existent-mcp",
			ToolName:  "test-tool",
			Arguments: map[string]any{},
		}

		callJSON, err := json.Marshal(toolCall)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader(callJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-admin-token")
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)

		var errorResponse map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse, "error")
		assert.Contains(t, errorResponse["error"], "MCP not found")
	})

	t.Run("Should validate tool call input", func(t *testing.T) {
		storage := mcpproxy.NewMemoryStorage()
		clientManager := mcpproxy.NewMockClientManager()

		config := &mcpproxy.Config{
			Port:            "8080",
			Host:            "localhost",
			BaseURL:         "http://localhost:8080",
			ShutdownTimeout: 5 * time.Second,
			AdminTokens:     []string{"test-admin-token"},
		}

		server := mcpproxy.NewServer(config, storage, clientManager)

		// Test invalid JSON
		req := httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-admin-token")
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test missing MCP name
		toolCall := mcpproxy.CallToolRequest{
			ToolName:  "test-tool",
			Arguments: map[string]any{},
		}

		callJSON, err := json.Marshal(toolCall)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader(callJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-admin-token")
		w = httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Test missing tool name
		toolCall = mcpproxy.CallToolRequest{
			MCPName:   "test-mcp",
			Arguments: map[string]any{},
		}

		callJSON, err = json.Marshal(toolCall)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader(callJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-admin-token")
		w = httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Should require authentication for tool endpoints", func(t *testing.T) {
		storage := mcpproxy.NewMemoryStorage()
		clientManager := mcpproxy.NewMockClientManager()

		config := &mcpproxy.Config{
			Port:            "8080",
			Host:            "localhost",
			BaseURL:         "http://localhost:8080",
			ShutdownTimeout: 5 * time.Second,
			AdminTokens:     []string{"valid-token"},
		}

		server := mcpproxy.NewServer(config, storage, clientManager)

		// Test tool listing without auth
		req := httptest.NewRequest(http.MethodGet, "/admin/tools", http.NoBody)
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// Test tool call without auth
		toolCall := mcpproxy.CallToolRequest{
			MCPName:   "test-mcp",
			ToolName:  "test-tool",
			Arguments: map[string]any{},
		}

		callJSON, err := json.Marshal(toolCall)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader(callJSON))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)

		// Test with invalid token
		req = httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader(callJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer invalid-token")
		w = httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
