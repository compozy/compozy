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
			Port:            "6001",
			Host:            "localhost",
			BaseURL:         "http://localhost:6001",
			ShutdownTimeout: 5 * time.Second,
		}

		server := mcpproxy.NewServer(config, storage, clientManager)

		// Test tool listing endpoint exists and responds with proper structure
		req := httptest.NewRequest(http.MethodGet, "/admin/tools", http.NoBody)
		// No authentication required anymore
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)

		// Should return 200 even with no tools
		assert.Equal(t, http.StatusOK, w.Code)

		var toolsResponse map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &toolsResponse)
		require.NoError(t, err)

		// Response should have tools array (empty or null is fine)
		tools, ok := toolsResponse["tools"]
		assert.True(t, ok, "Response should contain 'tools' field")
		// Tools can be null or empty array when no tools are available
		if tools != nil {
			// If not nil, should be an array
			_, isArray := tools.([]any)
			assert.True(t, isArray, "Tools field should be an array when not nil")
		}
	})

	t.Run("Should handle non-existent MCP in tool call", func(t *testing.T) {
		storage := mcpproxy.NewMemoryStorage()
		clientManager := mcpproxy.NewMockClientManager()

		config := &mcpproxy.Config{
			Port:            "6001",
			Host:            "localhost",
			BaseURL:         "http://localhost:6001",
			ShutdownTimeout: 5 * time.Second,
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
		// No authentication required anymore
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		var errorResponse map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse, "error")
		errorMsg, ok := errorResponse["error"].(string)
		require.True(t, ok, "error field should be a string")
		assert.Contains(t, errorMsg, "MCP not found")
	})

	t.Run("Should validate tool call input", func(t *testing.T) {
		storage := mcpproxy.NewMemoryStorage()
		clientManager := mcpproxy.NewMockClientManager()

		config := &mcpproxy.Config{
			Port:            "6001",
			Host:            "localhost",
			BaseURL:         "http://localhost:6001",
			ShutdownTimeout: 5 * time.Second,
		}

		server := mcpproxy.NewServer(config, storage, clientManager)

		// Test invalid JSON
		req := httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Validate error response structure
		var errorResponse map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse, "error", "Response should contain 'error' field")
		// Details field is optional for JSON parsing errors

		// Test missing MCP name
		toolCall := mcpproxy.CallToolRequest{
			ToolName:  "test-tool",
			Arguments: map[string]any{},
		}

		callJSON, err := json.Marshal(toolCall)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader(callJSON))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Validate error response structure
		err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse, "error", "Response should contain 'error' field")

		// Test missing tool name
		toolCall = mcpproxy.CallToolRequest{
			MCPName:   "test-mcp",
			Arguments: map[string]any{},
		}

		callJSON, err = json.Marshal(toolCall)
		require.NoError(t, err)

		req = httptest.NewRequest(http.MethodPost, "/admin/tools/call", bytes.NewReader(callJSON))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		server.Router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Validate error response structure
		err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse, "error", "Response should contain 'error' field")
	})
}

// IP allowlist enforcement tests were removed along with the feature.
